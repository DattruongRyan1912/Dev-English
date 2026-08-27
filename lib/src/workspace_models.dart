enum WorkspaceDataOrigin { demo, canonical }

extension WorkspaceDataOriginLabels on WorkspaceDataOrigin {
  String get label => switch (this) {
    WorkspaceDataOrigin.demo => 'Demo preview',
    WorkspaceDataOrigin.canonical => 'Canonical',
  };

  bool get isPreview => this == WorkspaceDataOrigin.demo;
}

class WorkspaceProject {
  const WorkspaceProject({
    required this.id,
    required this.name,
    required this.summary,
    required this.status,
  });

  final String id;
  final String name;
  final String summary;
  final String status;
}

class WorkspaceTask {
  const WorkspaceTask({
    required this.id,
    required this.title,
    required this.projectId,
    required this.status,
    required this.priority,
  });

  final String id;
  final String title;
  final String projectId;
  final String status;
  final String priority;
}

class WorkspaceDecision {
  const WorkspaceDecision({
    required this.id,
    required this.title,
    required this.outcome,
    required this.recordedAt,
  });

  final String id;
  final String title;
  final String outcome;
  final String recordedAt;
}

class KnowledgeClaim {
  const KnowledgeClaim({
    required this.id,
    required this.statement,
    required this.evidenceId,
    required this.confidence,
  });

  final String id;
  final String statement;
  final String evidenceId;
  final String confidence;
}

class KnowledgeSource {
  const KnowledgeSource({
    required this.id,
    required this.title,
    required this.kind,
    required this.updatedAt,
    required this.evidence,
    required this.claims,
  });

  final String id;
  final String title;
  final String kind;
  final String updatedAt;
  final List<KnowledgeEvidence> evidence;
  final List<KnowledgeClaim> claims;
}

class KnowledgeEvidence {
  const KnowledgeEvidence({
    required this.id,
    required this.location,
    required this.excerpt,
  });

  final String id;
  final String location;
  final String excerpt;
}

class AssistantCitation {
  const AssistantCitation({
    required this.sourceId,
    required this.sourceTitle,
    required this.location,
    required this.excerpt,
    required this.origin,
    this.stale = false,
  });

  final String sourceId;
  final String sourceTitle;
  final String location;
  final String excerpt;
  final WorkspaceDataOrigin origin;
  final bool stale;
}

class AssistantResponse {
  const AssistantResponse({
    required this.answer,
    required this.evidence,
    required this.unknowns,
    required this.staleSources,
    required this.suggestedActions,
    required this.actionReceipts,
  });

  final String answer;
  final List<AssistantCitation> evidence;
  final List<String> unknowns;
  final List<String> staleSources;
  final List<String> suggestedActions;
  final List<String> actionReceipts;
}

class AssistantTurn {
  const AssistantTurn({
    required this.id,
    required this.role,
    required this.content,
    this.citations = const [],
    this.unknowns = const [],
    this.suggestedActions = const [],
  });

  final String id;
  final String role;
  final String content;
  final List<AssistantCitation> citations;
  final List<String> unknowns;
  final List<String> suggestedActions;

  bool get isAssistant => role == 'assistant';
}

class WorkspaceData {
  const WorkspaceData({
    required this.displayName,
    required this.origin,
    required this.projects,
    required this.tasks,
    required this.decisions,
    required this.sources,
    required this.initialConversation,
  });

  final String displayName;
  final WorkspaceDataOrigin origin;
  final List<WorkspaceProject> projects;
  final List<WorkspaceTask> tasks;
  final List<WorkspaceDecision> decisions;
  final List<KnowledgeSource> sources;
  final List<AssistantTurn> initialConversation;

  WorkspaceProject projectFor(String id) => projects.firstWhere(
    (project) => project.id == id,
    orElse: () => projects.first,
  );

  KnowledgeSource? sourceFor(String id) {
    for (final source in sources) {
      if (source.id == id) return source;
    }
    return null;
  }
}
