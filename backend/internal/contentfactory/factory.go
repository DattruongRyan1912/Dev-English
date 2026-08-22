package contentfactory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Category string   `json:"category"`
	Levels   []string `json:"levels"`
}

type skillFile struct {
	Version string  `json:"version"`
	Skills  []Skill `json:"skills"`
}

type templateFile struct {
	Version   string   `json:"version"`
	Templates []string `json:"templates"`
}

type rolesFile struct {
	Version string   `json:"version"`
	Roles   []string `json:"roles"`
}

type rubricFile struct {
	Version string   `json:"version"`
	Rubrics []string `json:"rubrics"`
}

type grammarFile struct {
	Version  string   `json:"version"`
	Concepts []string `json:"concepts"`
}

type EvaluationCase struct {
	Name                    string `json:"name"`
	Mission                 string `json:"mission"`
	Answer                  string `json:"answer"`
	MinimumScore            int    `json:"minimum_score"`
	ExpectedCorrectionCount int    `json:"expected_correction_count"`
}

type Catalog struct {
	Skills      []Skill
	Templates   []string
	Roles       []string
	Rubrics     []string
	Grammar     []string
	PromptCount int
	SchemaCount int
	TestCases   []EvaluationCase
}

type Report struct {
	SkillFamilies       int `json:"skillFamilies"`
	DerivedSkillUnits   int `json:"derivedSkillUnits"`
	ExerciseTypes       int `json:"exerciseTypes"`
	Roles               int `json:"roles"`
	Rubrics             int `json:"rubrics"`
	GrammarConcepts     int `json:"grammarConcepts"`
	Prompts             int `json:"prompts"`
	Schemas             int `json:"schemas"`
	EvaluationCases     int `json:"evaluationCases"`
	GeneratedTestTarget int `json:"generatedTestTarget"`
}

func Load(root string) (Catalog, error) {
	var catalog Catalog
	if err := readJSON(filepath.Join(root, "content/taxonomy/skills.json"), &skillFile{Skills: nil}, func(value any) error {
		file := value.(*skillFile)
		catalog.Skills = file.Skills
		return nil
	}); err != nil {
		return Catalog{}, err
	}
	var templates templateFile
	if err := readJSONInto(filepath.Join(root, "content/templates/exercise-types.json"), &templates); err != nil {
		return Catalog{}, err
	}
	catalog.Templates = templates.Templates
	var roles rolesFile
	if err := readJSONInto(filepath.Join(root, "content/roles/roles.json"), &roles); err != nil {
		return Catalog{}, err
	}
	catalog.Roles = roles.Roles
	var rubrics rubricFile
	if err := readJSONInto(filepath.Join(root, "content/rubrics/rubrics.json"), &rubrics); err != nil {
		return Catalog{}, err
	}
	catalog.Rubrics = rubrics.Rubrics
	var grammar grammarFile
	if err := readJSONInto(filepath.Join(root, "content/grammar/concepts.json"), &grammar); err != nil {
		return Catalog{}, err
	}
	catalog.Grammar = grammar.Concepts
	_ = filepath.Walk(filepath.Join(root, "prompts"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
			catalog.PromptCount++
		}
		return nil
	})
	_ = filepath.Walk(filepath.Join(root, "schemas"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".json") {
			catalog.SchemaCount++
		}
		return nil
	})
	evaluationPath := filepath.Join(root, "tests/evaluation_cases.generated.json")
	if _, err := os.Stat(evaluationPath); err != nil {
		evaluationPath = filepath.Join(root, "tests/evaluation_cases.json")
	}
	if err := readJSONInto(evaluationPath, &catalog.TestCases); err != nil {
		return Catalog{}, err
	}
	return catalog, Validate(catalog)
}

// Validate keeps the factory deterministic and catches duplicate IDs before
// content can reach the database. AI generation is never part of this step.
func Validate(catalog Catalog) error {
	if len(catalog.Skills) == 0 || len(catalog.Templates) == 0 {
		return errors.New("skill and exercise catalogs are required")
	}
	if err := uniqueStrings("skill", skillIDs(catalog.Skills)); err != nil {
		return err
	}
	if err := uniqueStrings("template", catalog.Templates); err != nil {
		return err
	}
	if err := uniqueStrings("role", catalog.Roles); err != nil {
		return err
	}
	if len(catalog.Templates) < 30 {
		return fmt.Errorf("at least 30 exercise types are required, got %d", len(catalog.Templates))
	}
	if len(catalog.Roles) < 20 {
		return fmt.Errorf("at least 20 roleplay roles are required, got %d", len(catalog.Roles))
	}
	if len(catalog.Rubrics) < 3 || len(catalog.Grammar) < 10 {
		return errors.New("rubric and grammar catalogs are incomplete")
	}
	if len(catalog.TestCases) < 3 {
		return errors.New("evaluation test cases are required")
	}
	for _, item := range catalog.TestCases {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Answer) == "" {
			return errors.New("evaluation cases require a name and answer")
		}
	}
	return nil
}

func BuildReport(catalog Catalog) Report {
	derived := 0
	for _, skill := range catalog.Skills {
		// Each skill/CEFR pair is split into task, language, vocabulary and
		// reflection capabilities before seed generation.
		derived += len(skill.Levels) * 4
	}
	return Report{SkillFamilies: len(catalog.Skills), DerivedSkillUnits: derived, ExerciseTypes: len(catalog.Templates), Roles: len(catalog.Roles), Rubrics: len(catalog.Rubrics), GrammarConcepts: len(catalog.Grammar), Prompts: catalog.PromptCount, Schemas: catalog.SchemaCount, EvaluationCases: len(catalog.TestCases), GeneratedTestTarget: 200}
}

func GeneratedEvaluationCases(target int) []EvaluationCase {
	if target < 1 {
		return nil
	}
	seeds := []EvaluationCase{
		{Name: "complete_bug_report", Mission: "Write a bug report for an API returning HTTP 500.", Answer: "The API returns a 500 error for large uploads. The expected behavior is a validation error. This impacts users, and the next step is to reproduce the issue.", MinimumScore: 70},
		{Name: "article_mistake", Mission: "Write a concise bug report.", Answer: "I created bug report and discuss about the impact.", ExpectedCorrectionCount: 2},
		{Name: "short_answer_needs_retry", Mission: "Explain the API failure.", Answer: "It fails sometimes."},
	}
	cases := make([]EvaluationCase, 0, target)
	for index := 0; index < target; index++ {
		seed := seeds[index%len(seeds)]
		seed.Name = fmt.Sprintf("%s_%03d", seed.Name, index+1)
		cases = append(cases, seed)
	}
	return cases
}

func WriteEvaluationCases(path string, target int) error {
	cases := GeneratedEvaluationCases(target)
	payload, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func readJSON(path string, target any, assign func(any) error) error {
	if err := readJSONInto(path, target); err != nil {
		return err
	}
	return assign(target)
}

func readJSONInto(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func uniqueStrings(label string, values []string) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s cannot be empty", label)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("duplicate %s %q", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func skillIDs(skills []Skill) []string {
	ids := make([]string, 0, len(skills))
	for _, skill := range skills {
		ids = append(ids, skill.ID)
	}
	sort.Strings(ids)
	return ids
}
