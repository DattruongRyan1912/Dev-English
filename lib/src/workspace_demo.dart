import 'workspace_models.dart';

class WorkspaceDemo {
  static const source = KnowledgeSource(
    id: 'source-wave3-plan',
    title: 'DevEnglish V1 delivery plan',
    kind: 'Project document',
    updatedAt: 'Updated today',
    evidence: [
      KnowledgeEvidence(
        id: 'evidence-wave3-mcp',
        location: 'Wave 3 / TASK-007',
        excerpt:
            'MCP read tools reuse the same application services as REST and keep write tools behind explicit confirmation.',
      ),
      KnowledgeEvidence(
        id: 'evidence-wave3-ui',
        location: 'Wave 3 / TASK-008',
        excerpt:
            'Today is the default workspace with Work, Knowledge and Learning available as separate destinations.',
      ),
      KnowledgeEvidence(
        id: 'evidence-wave3-task',
        location: 'Wave 3 / TASK-008',
        excerpt:
            'The current high-priority task is to finish the Today, Work and Knowledge workspace UI and validate text assistant citations before adding voice features.',
      ),
    ],
    claims: [
      KnowledgeClaim(
        id: 'claim-wave3-ui',
        statement: 'Today is the default workspace for the assistant.',
        evidenceId: 'evidence-wave3-ui',
        confidence: 'preview',
      ),
      KnowledgeClaim(
        id: 'claim-wave3-mcp',
        statement: 'MCP and REST should share application-service semantics.',
        evidenceId: 'evidence-wave3-mcp',
        confidence: 'preview',
      ),
    ],
  );

  static const data = WorkspaceData(
    displayName: 'Ryan',
    origin: WorkspaceDataOrigin.demo,
    projects: [
      WorkspaceProject(
        id: 'project-devenglish',
        name: 'Dev-English',
        summary: 'AI work assistant with a practical English layer.',
        status: 'Active',
      ),
    ],
    tasks: [
      WorkspaceTask(
        id: 'task-008',
        title: 'Build Today, Work and Knowledge workspace UI',
        projectId: 'project-devenglish',
        status: 'In progress',
        priority: 'High',
      ),
      WorkspaceTask(
        id: 'task-grounding',
        title: 'Keep assistant answers tied to source evidence',
        projectId: 'project-devenglish',
        status: 'Next',
        priority: 'High',
      ),
    ],
    decisions: [
      WorkspaceDecision(
        id: 'decision-compiled-monolith',
        title: 'Use a compiled modular monolith for V1',
        outcome:
            'Modules share one Go binary; runtime code loading is out of scope.',
        recordedAt: 'Recorded in architecture plan',
      ),
      WorkspaceDecision(
        id: 'decision-source-trust',
        title: 'Canonical facts require source evidence',
        outcome:
            'Without evidence, the assistant must say unknown or inferred.',
        recordedAt: 'Recorded in data integrity rules',
      ),
    ],
    sources: [source],
    initialConversation: [
      AssistantTurn(
        id: 'welcome',
        role: 'assistant',
        content:
            'I can help you move the DevEnglish project forward. Ask about a task, decision or source-backed project detail.',
      ),
    ],
  );
}
