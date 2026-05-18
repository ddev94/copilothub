package hub

import "net/http"

type Registry struct {
	features []Feature
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(f Feature) {
	r.features = append(r.features, f)
}

func (r *Registry) RegisterRoutes(mux *http.ServeMux, ctx FeatureContext) {
	for _, f := range r.features {
		if err := f.Init(ctx); err != nil {
			continue
		}
		sub := http.NewServeMux()
		f.RegisterRoutes(sub)
		prefix := "/api/features/" + f.ID()
		mux.Handle(prefix+"/", http.StripPrefix(prefix, sub))

		// External features also serve a frontend UI
		if ef, ok := f.(*ExternalFeature); ok {
			frontendPrefix := "/features/" + f.ID()
			mux.Handle(frontendPrefix+"/", http.StripPrefix(frontendPrefix, ef.FrontendHandler()))
		}
	}
}

func (r *Registry) Manifests() []Manifest {
	m := make([]Manifest, len(r.features))
	for i, f := range r.features {
		m[i] = f.Manifest()
	}
	return m
}
