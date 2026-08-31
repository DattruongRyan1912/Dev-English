package work

import "testing"

func TestNewServiceRejectsNilRepositoryAndIgnoresNilOptions(t *testing.T) {
	if service, err := NewService(nil); err == nil || service != nil {
		t.Fatalf("NewService(nil) = service=%v, err=%v; want nil service and validation error", service, err)
	}

	service, err := NewService(NewMemoryRepository(), nil, WithClock(nil))
	if err != nil || service == nil {
		t.Fatalf("NewService with nil options = service=%v, err=%v; want a service", service, err)
	}

	defaultClockService, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatalf("NewService without options: %v", err)
	}
	project, err := defaultClockService.CreateProject(testContext, testScope("default-clock-workspace", "default-clock-user"), CreateProjectInput{Name: "Default clock"}, "default-clock-create")
	if err != nil || project.CreatedAt.IsZero() || project.UpdatedAt.IsZero() {
		t.Fatalf("default clock project = %+v, err=%v; want non-zero timestamps", project, err)
	}
}
