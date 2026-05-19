"""
Knowledge sidecar service – LangChain + ChromaDB + paraphrase-multilingual-MiniLM-L12-v2.

Endpoints:
  POST /ingest           multipart: file, projectId, fileName
  GET  /documents        ?projectId=...
  DELETE /documents/{id} ?projectId=...
  POST /retrieve         {projectId, query, topK}
"""

import hashlib
import os
import shutil
import tempfile
import time
import uuid
from pathlib import Path
from typing import List

import docx2txt
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse
from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_chroma import Chroma
from langchain_community.document_loaders import PyPDFLoader, TextLoader
from langchain_community.embeddings import HuggingFaceEmbeddings
from pydantic import BaseModel

# ---------------------------------------------------------------------------
# Embedding model (loaded once at startup)
# ---------------------------------------------------------------------------
EMBEDDING_MODEL = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
_embeddings: HuggingFaceEmbeddings | None = None


def get_embeddings() -> HuggingFaceEmbeddings:
    global _embeddings
    if _embeddings is None:
        print(f"Loading embedding model: {EMBEDDING_MODEL}")
        _embeddings = HuggingFaceEmbeddings(
            model_name=EMBEDDING_MODEL,
            model_kwargs={"device": "cpu"},
            encode_kwargs={"normalize_embeddings": True},
        )
        print("Embedding model ready.")
    return _embeddings


# ---------------------------------------------------------------------------
# ChromaDB storage
# ---------------------------------------------------------------------------
CHROMA_DIR = Path(os.getenv("CHROMA_DIR", "./chroma_db"))
CHROMA_COLLECTION = "knowledge"


def get_vectorstore(project_id: str) -> Chroma:
    persist_dir = str(CHROMA_DIR / project_id)
    return Chroma(
        collection_name=CHROMA_COLLECTION,
        embedding_function=get_embeddings(),
        persist_directory=persist_dir,
    )


# ---------------------------------------------------------------------------
# Text extraction helpers
# ---------------------------------------------------------------------------

def extract_text(file_path: str, filename: str) -> str:
    ext = Path(filename).suffix.lower()
    if ext == ".pdf":
        loader = PyPDFLoader(file_path)
        pages = loader.load()
        return "\n\n".join(p.page_content for p in pages)
    if ext == ".docx":
        return docx2txt.process(file_path)
    # .md and plain text
    loader = TextLoader(file_path, encoding="utf-8")
    docs = loader.load()
    return "\n\n".join(d.page_content for d in docs)


# ---------------------------------------------------------------------------
# In-memory document metadata index (per process)
# A lightweight alternative to a full metadata DB; persisted to chroma metadata.
# ---------------------------------------------------------------------------

app = FastAPI(title="Knowledge Service")


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------

@app.post("/ingest")
async def ingest(
    file: UploadFile = File(...),
    projectId: str = Form(...),
    fileName: str = Form(""),
):
    original_name = fileName or file.filename or "upload"
    ext = Path(original_name).suffix.lower()
    if ext not in {".pdf", ".md", ".docx"}:
        raise HTTPException(status_code=400, detail=f"Unsupported file type: {ext}")

    # Save upload to temp file
    with tempfile.NamedTemporaryFile(delete=False, suffix=ext) as tmp:
        content = await file.read()
        tmp.write(content)
        tmp_path = tmp.name

    try:
        text = extract_text(tmp_path, original_name)
    except Exception as e:
        os.unlink(tmp_path)
        raise HTTPException(status_code=422, detail=f"Text extraction failed: {e}")
    finally:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)

    if not text.strip():
        raise HTTPException(status_code=422, detail="No text could be extracted from the file.")

    # Chunk
    splitter = RecursiveCharacterTextSplitter(chunk_size=800, chunk_overlap=100)
    chunks = splitter.split_text(text)

    doc_id = str(uuid.uuid4())
    sha256 = hashlib.sha256(content).hexdigest()
    created_at = str(int(time.time()))

    vs = get_vectorstore(projectId)
    ids = [f"{doc_id}_{i}" for i in range(len(chunks))]
    metadatas = [
        {
            "doc_id": doc_id,
            "source_file": original_name,
            "chunk_index": i,
            "sha256": sha256,
            "created_at": created_at,
        }
        for i in range(len(chunks))
    ]
    vs.add_texts(texts=chunks, metadatas=metadatas, ids=ids)

    return {"ok": True, "docId": doc_id, "chunks": len(chunks)}


@app.get("/documents")
async def list_documents(projectId: str):
    vs = get_vectorstore(projectId)
    col = vs._collection
    result = col.get(include=["metadatas"])
    seen: dict[str, dict] = {}
    for meta in result.get("metadatas") or []:
        if not meta:
            continue
        doc_id = meta.get("doc_id", "")
        if doc_id and doc_id not in seen:
            seen[doc_id] = {
                "id": doc_id,
                "name": meta.get("source_file", ""),
                "sourceFile": meta.get("source_file", ""),
                "createdAt": _unix_to_iso(meta.get("created_at", "0")),
            }
    return {"documents": list(seen.values())}


@app.delete("/documents/{doc_id}")
async def delete_document(doc_id: str, projectId: str):
    vs = get_vectorstore(projectId)
    col = vs._collection
    result = col.get(where={"doc_id": doc_id}, include=["metadatas"])
    ids = result.get("ids") or []
    if not ids:
        raise HTTPException(status_code=404, detail="Document not found")
    col.delete(ids=ids)
    return {"ok": True, "deleted": len(ids)}


class RetrieveRequest(BaseModel):
    projectId: str
    query: str
    topK: int = 6


@app.post("/retrieve")
async def retrieve(req: RetrieveRequest):
    vs = get_vectorstore(req.projectId)
    results = vs.similarity_search_with_relevance_scores(req.query, k=req.topK)
    chunks = [
        {"content": doc.page_content, "score": float(score)}
        for doc, score in results
    ]
    return {"chunks": chunks}


@app.get("/health")
async def health():
    return {"status": "ok"}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _unix_to_iso(ts: str) -> str:
    try:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(int(ts)))
    except (ValueError, OSError):
        return ts


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    import uvicorn

    port = int(os.getenv("PORT", "8001"))
    uvicorn.run("main:app", host="0.0.0.0", port=port, reload=False)
