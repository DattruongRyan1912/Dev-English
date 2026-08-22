package contentfactory

import "testing"

func TestGeneratedEvaluationCasesMeetFactoryTarget(t *testing.T) {
	cases := GeneratedEvaluationCases(200)
	if len(cases) != 200 {
		t.Fatalf("got %d cases, want 200", len(cases))
	}
	if cases[0].Name == cases[1].Name {
		t.Fatal("generated case names must be unique")
	}
}

func TestValidateRejectsDuplicateTemplates(t *testing.T) {
	catalog := Catalog{Skills: []Skill{{ID: "skill", Levels: []string{"A1"}}}, Templates: []string{"same", "same"}, Roles: make([]string, 20), Rubrics: []string{"a", "b", "c"}, Grammar: make([]string, 10), TestCases: []EvaluationCase{{Name: "case", Answer: "answer"}}}
	if err := Validate(catalog); err == nil {
		t.Fatal("expected duplicate template error")
	}
}
