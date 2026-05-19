package spec

import "time"

type Spec struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Requirement string      `json:"requirement"`
	UserStories []UserStory `json:"userStories"`
}

type SpecMeta struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UserStory struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	Story              string                `json:"story"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptanceCriteria"`
	TestCases          []TestCase            `json:"testCases"`
}

type AcceptanceCriterion struct {
	ID    string `json:"id"`
	Given string `json:"given"`
	When  string `json:"when"`
	Then  string `json:"then"`
}

type TestCase struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Steps          string `json:"steps"`
	ExpectedResult string `json:"expectedResult"`
}
