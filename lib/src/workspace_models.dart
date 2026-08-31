enum WorkspaceDataOrigin { demo, canonical }

extension WorkspaceDataOriginLabels on WorkspaceDataOrigin {
  String get label => switch (this) {
    WorkspaceDataOrigin.demo => 'Demo preview',
    WorkspaceDataOrigin.canonical => 'Canonical',
  };

  bool get isPreview => this == WorkspaceDataOrigin.demo;
}

/// A small canonical reference used when opening the assistant from a work or
/// knowledge item. The label is presentation-only; the backend resolves the
/// type/id inside the authenticated workspace before retrieval.
class WorkspaceAssistantContext {
  const WorkspaceAssistantContext({
    required this.entityType,
    required this.entityId,
    required this.label,
  });

  final String entityType;
  final String entityId;
  final String label;

  bool get isEmpty => entityType.trim().isEmpty || entityId.trim().isEmpty;

  factory WorkspaceAssistantContext.fromJson(Map<String, dynamic> json) {
    final type = _string(json['type'], '');
    final id = _string(json['id'], '');
    return WorkspaceAssistantContext(
      entityType: type,
      entityId: id,
      label: _string(json['label'], '$type · $id'),
    );
  }

  Map<String, dynamic> toRequestJson() => {
    'contextType': entityType,
    'contextId': entityId,
  };

  @override
  bool operator ==(Object other) =>
      other is WorkspaceAssistantContext &&
      other.entityType == entityType &&
      other.entityId == entityId;

  @override
  int get hashCode => Object.hash(entityType, entityId);
}

class WorkspaceProject {
  const WorkspaceProject({
    required this.id,
    required this.name,
    required this.summary,
    required this.status,
    this.version = 1,
    this.deletedAt,
    this.origin = 'canonical',
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String name;
  final String summary;
  final String status;
  final int version;
  final DateTime? deletedAt;
  final String origin;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  factory WorkspaceProject.fromJson(Map<String, dynamic> json) =>
      WorkspaceProject(
        id: _string(json['id'], 'project-unknown'),
        name: _string(json['name'], 'Untitled project'),
        summary: _string(json['description'], 'No description yet.'),
        status: _label(json['status']),
        version: _int(json['version'], 1),
        deletedAt: _date(json['deletedAt']),
        origin: _string(json['origin'], 'canonical'),
        createdAt: _date(json['createdAt']),
        updatedAt: _date(json['updatedAt']),
      );
}

class WorkspaceTask {
  const WorkspaceTask({
    required this.id,
    required this.title,
    required this.projectId,
    required this.status,
    required this.priority,
    this.description = '',
    this.version = 1,
    this.deletedAt,
    this.origin = 'canonical',
    this.dueAt,
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String title;
  final String projectId;
  final String status;
  final String priority;
  final String description;
  final int version;
  final DateTime? deletedAt;
  final String origin;
  final DateTime? dueAt;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  factory WorkspaceTask.fromJson(Map<String, dynamic> json) => WorkspaceTask(
    id: _string(json['id'], 'task-unknown'),
    title: _string(json['title'], 'Untitled task'),
    projectId: _string(json['projectId'], ''),
    status: _label(json['status']),
    priority: _label(json['priority']),
    description: _string(json['description'], ''),
    version: _int(json['version'], 1),
    deletedAt: _date(json['deletedAt']),
    origin: _string(json['origin'], 'canonical'),
    dueAt: _date(json['dueAt']),
    createdAt: _date(json['createdAt']),
    updatedAt: _date(json['updatedAt']),
  );
}

class WorkspaceDecision {
  const WorkspaceDecision({
    required this.id,
    required this.title,
    required this.outcome,
    required this.recordedAt,
    this.projectId = '',
    this.context = '',
    this.rationale = '',
    this.status = 'Unknown',
    this.version = 1,
    this.deletedAt,
    this.origin = 'canonical',
  });

  final String id;
  final String title;
  final String outcome;
  final String recordedAt;
  final String projectId;
  final String context;
  final String rationale;
  final String status;
  final int version;
  final DateTime? deletedAt;
  final String origin;

  factory WorkspaceDecision.fromJson(Map<String, dynamic> json) =>
      WorkspaceDecision(
        id: _string(json['id'], 'decision-unknown'),
        title: _string(json['title'], 'Untitled decision'),
        outcome: _string(json['outcome'], _string(json['context'], '')),
        recordedAt: _string(
          json['updatedAt'],
          _string(json['createdAt'], 'Recorded recently'),
        ),
        projectId: _string(json['projectId'], ''),
        context: _string(json['context'], ''),
        rationale: _string(json['rationale'], ''),
        status: _label(json['status']),
        version: _int(json['version'], 1),
        deletedAt: _date(json['deletedAt']),
        origin: _string(json['origin'], 'canonical'),
      );
}

/// Immutable audit entry returned by the canonical Work history endpoints.
///
/// The UI intentionally renders the action and version transition instead of
/// exposing the raw before/after snapshots. Those snapshots remain owned by
/// the backend audit record and are not canonical presentation data.
class WorkspaceHistoryEvent {
  const WorkspaceHistoryEvent({
    required this.id,
    required this.workspaceId,
    required this.actorUserId,
    required this.entityType,
    required this.entityId,
    required this.action,
    required this.fromVersion,
    required this.toVersion,
    required this.idempotencyKey,
    required this.createdAt,
  });

  final String id;
  final String workspaceId;
  final String actorUserId;
  final String entityType;
  final String entityId;
  final String action;
  final int fromVersion;
  final int toVersion;
  final String idempotencyKey;
  final DateTime? createdAt;

  factory WorkspaceHistoryEvent.fromJson(Map<String, dynamic> json) =>
      WorkspaceHistoryEvent(
        id: _string(json['id'], 'history-unknown'),
        workspaceId: _string(json['workspaceId'], ''),
        actorUserId: _string(json['actorUserId'], ''),
        entityType: _string(json['entityType'], 'work item'),
        entityId: _string(json['entityId'], ''),
        action: _label(json['action']),
        fromVersion: _int(json['fromVersion'], 0),
        toVersion: _int(json['toVersion'], 0),
        idempotencyKey: _string(json['idempotencyKey'], ''),
        createdAt: _date(json['createdAt']),
      );
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

  factory KnowledgeClaim.fromJson(Map<String, dynamic> json) => KnowledgeClaim(
    id: _string(json['id'], 'claim-unknown'),
    statement: _string(json['statement'], ''),
    evidenceId: _string(json['evidenceId'], ''),
    confidence: _label(json['certainty'] ?? json['confidence']),
  );
}

class KnowledgeSource {
  const KnowledgeSource({
    required this.id,
    required this.title,
    required this.kind,
    required this.updatedAt,
    required this.evidence,
    required this.claims,
    this.uri = '',
    this.version = 1,
    this.deletedAt,
  });

  final String id;
  final String title;
  final String kind;
  final String updatedAt;
  final List<KnowledgeEvidence> evidence;
  final List<KnowledgeClaim> claims;
  final String uri;
  final int version;
  final DateTime? deletedAt;

  factory KnowledgeSource.fromJson(Map<String, dynamic> json) =>
      KnowledgeSource(
        id: _string(json['id'], 'source-unknown'),
        title: _string(json['title'], _string(json['name'], 'Untitled source')),
        kind: _label(json['kind']),
        updatedAt: _string(
          json['updatedAt'],
          _string(json['updated_at'], 'Updated recently'),
        ),
        uri: _string(json['uri'], ''),
        version: _int(json['version'], 1),
        deletedAt: _date(json['deletedAt']),
        evidence: _maps(
          json['evidence'],
        ).map(KnowledgeEvidence.fromJson).toList(growable: false),
        claims: _maps(
          json['claims'],
        ).map(KnowledgeClaim.fromJson).toList(growable: false),
      );
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

  factory KnowledgeEvidence.fromJson(Map<String, dynamic> json) =>
      KnowledgeEvidence(
        id: _string(
          json['id'],
          _string(json['evidenceId'], 'evidence-unknown'),
        ),
        location: _string(
          json['location'],
          _string(json['locator'], _string(json['uri'], 'Source excerpt')),
        ),
        excerpt: _string(json['excerpt'], _string(json['quote'], '')),
      );
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

  factory AssistantCitation.fromJson(
    Map<String, dynamic> json, {
    WorkspaceDataOrigin origin = WorkspaceDataOrigin.canonical,
    bool stale = false,
  }) => AssistantCitation(
    sourceId: _string(json['sourceId'], _string(json['evidenceId'], '')),
    sourceTitle: _string(
      json['sourceTitle'],
      _string(json['sourceName'], 'Verified evidence'),
    ),
    location: _string(
      json['location'],
      _string(json['locator'], 'Source excerpt'),
    ),
    excerpt: _string(json['excerpt'], _string(json['quote'], '')),
    origin: origin,
    stale: stale,
  );
}

class AssistantSuggestedAction {
  const AssistantSuggestedAction({
    required this.id,
    required this.kind,
    required this.label,
    required this.target,
    required this.requiresConfirmation,
  });

  final String id;
  final String kind;
  final String label;
  final String target;
  final bool requiresConfirmation;

  factory AssistantSuggestedAction.fromJson(Map<String, dynamic> json) =>
      AssistantSuggestedAction(
        id: _string(json['id'], ''),
        kind: _string(json['kind'], 'suggested.action'),
        label: _string(json['label'], 'Suggested action'),
        target: _string(json['target'], ''),
        requiresConfirmation: json['requiresConfirmation'] != false,
      );
}

class AssistantActionReceipt {
  const AssistantActionReceipt({
    required this.id,
    required this.actionId,
    required this.provider,
    required this.operation,
    required this.challengeId,
    required this.targetId,
    required this.status,
    required this.replayed,
  });

  final String id;
  final String actionId;
  final String provider;
  final String operation;
  final String challengeId;
  final String targetId;
  final String status;
  final bool replayed;

  factory AssistantActionReceipt.fromJson(Map<String, dynamic> json) =>
      AssistantActionReceipt(
        id: _string(json['id'], 'receipt-unknown'),
        actionId: _string(json['actionId'], ''),
        provider: _string(json['provider'], ''),
        operation: _string(json['operation'], 'action'),
        challengeId: _string(json['challengeId'], ''),
        targetId: _string(json['targetId'], ''),
        status: _string(json['status'], 'unknown'),
        replayed: json['replayed'] == true,
      );
}

class AssistantResponse {
  const AssistantResponse({
    required this.answer,
    required this.evidence,
    required this.unknowns,
    required this.staleSources,
    required this.suggestedActions,
    required this.actionReceipts,
    this.grounding = 'unknown',
    this.actionDetails = const [],
    this.receiptDetails = const [],
  });

  final String answer;
  final List<AssistantCitation> evidence;
  final List<String> unknowns;
  final List<String> staleSources;
  final List<String> suggestedActions;
  final List<String> actionReceipts;
  final String grounding;
  final List<AssistantSuggestedAction> actionDetails;
  final List<AssistantActionReceipt> receiptDetails;

  factory AssistantResponse.fromJson(Map<String, dynamic> json) {
    final payload = _map(json['response']).isNotEmpty
        ? _map(json['response'])
        : json;
    final staleSources = _strings(payload['staleSources']);
    final suggestedActionMaps = _maps(payload['suggestedActions']);
    final actionDetails = suggestedActionMaps
        .map(AssistantSuggestedAction.fromJson)
        .toList(growable: false);
    final receiptMaps = _maps(payload['actionReceipts']);
    final receiptDetails = receiptMaps
        .map(AssistantActionReceipt.fromJson)
        .toList(growable: false);
    const origin = WorkspaceDataOrigin.canonical;
    return AssistantResponse(
      answer: _string(
        payload['answer'],
        'I do not have a verified answer yet.',
      ),
      evidence: _maps(payload['evidence'])
          .map(
            (item) => AssistantCitation.fromJson(
              item,
              origin: origin,
              stale: staleSources.contains(_string(item['evidenceId'], '')),
            ),
          )
          .toList(growable: false),
      unknowns: _strings(payload['unknowns']),
      staleSources: staleSources,
      suggestedActions: actionDetails.isEmpty
          ? _strings(payload['suggestedActions'])
          : actionDetails.map((item) => item.label).toList(growable: false),
      actionReceipts: receiptDetails.isEmpty
          ? _strings(payload['actionReceipts'])
          : receiptDetails.map((item) => item.id).toList(growable: false),
      grounding: _string(payload['grounding'], 'unknown'),
      actionDetails: actionDetails,
      receiptDetails: receiptDetails,
    );
  }
}

class WorkspaceActionTarget {
  const WorkspaceActionTarget({
    required this.operation,
    this.repository = '',
    this.issue = 0,
    this.title = '',
    this.body = '',
    this.labels = const [],
    this.expectedRevision = '',
    this.entityType = '',
    this.entityId = '',
    this.expectedVersion = 0,
  });

  final String operation;
  final String repository;
  final int issue;
  final String title;
  final String body;
  final List<String> labels;
  final String expectedRevision;
  final String entityType;
  final String entityId;
  final int expectedVersion;

  factory WorkspaceActionTarget.fromJson(Map<String, dynamic> json) =>
      WorkspaceActionTarget(
        operation: _string(json['operation'], ''),
        repository: _string(json['repository'], ''),
        issue: _int(json['issue'], 0),
        title: _string(json['title'], ''),
        body: _string(json['body'], ''),
        labels: _strings(json['labels']),
        expectedRevision: _string(json['expectedRevision'], ''),
        entityType: _string(json['entityType'], ''),
        entityId: _string(json['entityId'], ''),
        expectedVersion: _int(json['expectedVersion'], 0),
      );

  Map<String, dynamic> toJson() => <String, dynamic>{
    'operation': operation,
    'repository': repository,
    if (issue > 0) 'issue': issue,
    if (title.isNotEmpty) 'title': title,
    if (body.isNotEmpty) 'body': body,
    if (labels.isNotEmpty) 'labels': labels,
    if (expectedRevision.isNotEmpty) 'expectedRevision': expectedRevision,
    if (entityType.isNotEmpty) 'entityType': entityType,
    if (entityId.isNotEmpty) 'entityId': entityId,
    if (expectedVersion > 0) 'expectedVersion': expectedVersion,
  };
}

class WorkspaceActionBinding {
  const WorkspaceActionBinding({
    required this.actionId,
    required this.provider,
    required this.operation,
    required this.challengeId,
    required this.idempotencyKey,
    required this.actionHash,
    required this.targetType,
    required this.targetId,
  });

  final String actionId;
  final String provider;
  final String operation;
  final String challengeId;
  final String idempotencyKey;
  final String actionHash;
  final String targetType;
  final String targetId;

  factory WorkspaceActionBinding.fromJson(Map<String, dynamic> json) =>
      WorkspaceActionBinding(
        actionId: _string(json['actionId'], ''),
        provider: _string(json['provider'], ''),
        operation: _string(json['operation'], ''),
        challengeId: _string(json['challengeId'], ''),
        idempotencyKey: _string(json['idempotencyKey'], ''),
        actionHash: _string(json['actionHash'], ''),
        targetType: _string(json['targetType'], ''),
        targetId: _string(json['targetId'], ''),
      );
}

class WorkspaceActionChallenge {
  const WorkspaceActionChallenge({
    required this.action,
    required this.target,
    required this.expiresAt,
  });

  final WorkspaceActionBinding action;
  final WorkspaceActionTarget target;
  final DateTime? expiresAt;

  factory WorkspaceActionChallenge.fromJson(Map<String, dynamic> json) =>
      WorkspaceActionChallenge(
        action: WorkspaceActionBinding.fromJson(_map(json['action'])),
        target: WorkspaceActionTarget.fromJson(_map(json['target'])),
        expiresAt: _date(json['expiresAt']),
      );
}

class WorkspaceActionConfirmation {
  const WorkspaceActionConfirmation({
    required this.action,
    required this.receipt,
    required this.replayed,
  });

  final WorkspaceActionBinding action;
  final AssistantActionReceipt receipt;
  final bool replayed;

  factory WorkspaceActionConfirmation.fromJson(Map<String, dynamic> json) =>
      WorkspaceActionConfirmation(
        action: WorkspaceActionBinding.fromJson(_map(json['action'])),
        receipt: AssistantActionReceipt.fromJson(_map(json['receipt'])),
        replayed: json['replayed'] == true,
      );
}

class KnowledgeSourceDetail {
  const KnowledgeSourceDetail({
    required this.source,
    required this.items,
    required this.revisions,
    required this.chunks,
    required this.evidence,
  });

  final KnowledgeSource source;
  final List<KnowledgeSourceItem> items;
  final List<KnowledgeRevision> revisions;
  final List<KnowledgeChunk> chunks;
  final List<KnowledgeEvidence> evidence;

  factory KnowledgeSourceDetail.fromJson(Map<String, dynamic> json) {
    return KnowledgeSourceDetail(
      source: KnowledgeSource.fromJson(_map(json['source'])),
      items: _maps(
        json['items'],
      ).map(KnowledgeSourceItem.fromJson).toList(growable: false),
      revisions: _maps(
        json['revisions'],
      ).map(KnowledgeRevision.fromJson).toList(growable: false),
      chunks: _maps(
        json['chunks'],
      ).map(KnowledgeChunk.fromJson).toList(growable: false),
      evidence: _maps(
        json['evidence'],
      ).map(KnowledgeEvidence.fromJson).toList(growable: false),
    );
  }
}

class KnowledgeSourceItem {
  const KnowledgeSourceItem({
    required this.id,
    required this.title,
    required this.uri,
    required this.mimeType,
    required this.currentRevisionId,
    required this.version,
  });

  final String id;
  final String title;
  final String uri;
  final String mimeType;
  final String currentRevisionId;
  final int version;

  factory KnowledgeSourceItem.fromJson(Map<String, dynamic> json) =>
      KnowledgeSourceItem(
        id: _string(json['id'], 'item-unknown'),
        title: _string(json['title'], 'Untitled item'),
        uri: _string(json['uri'], ''),
        mimeType: _string(json['mimeType'], ''),
        currentRevisionId: _string(json['currentRevisionId'], ''),
        version: _int(json['version'], 1),
      );
}

class KnowledgeRevision {
  const KnowledgeRevision({
    required this.id,
    required this.sourceItemId,
    required this.revisionKey,
    required this.contentHash,
    required this.content,
    required this.sourceUri,
    required this.ingestedAt,
  });

  final String id;
  final String sourceItemId;
  final String revisionKey;
  final String contentHash;
  final String content;
  final String sourceUri;
  final DateTime? ingestedAt;

  factory KnowledgeRevision.fromJson(Map<String, dynamic> json) =>
      KnowledgeRevision(
        id: _string(json['id'], 'revision-unknown'),
        sourceItemId: _string(json['sourceItemId'], ''),
        revisionKey: _string(json['revisionKey'], ''),
        contentHash: _string(json['contentHash'], ''),
        content: _string(json['content'], ''),
        sourceUri: _string(json['sourceUri'], ''),
        ingestedAt: _date(json['ingestedAt']),
      );
}

class KnowledgeChunk {
  const KnowledgeChunk({
    required this.id,
    required this.revisionId,
    required this.ordinal,
    required this.text,
    required this.tokenCount,
  });

  final String id;
  final String revisionId;
  final int ordinal;
  final String text;
  final int tokenCount;

  factory KnowledgeChunk.fromJson(Map<String, dynamic> json) => KnowledgeChunk(
    id: _string(json['id'], 'chunk-unknown'),
    revisionId: _string(json['revisionId'], ''),
    ordinal: _int(json['ordinal'], 0),
    text: _string(json['text'], ''),
    tokenCount: _int(json['tokenCount'], 0),
  );
}

class AssistantTurn {
  const AssistantTurn({
    required this.id,
    required this.role,
    required this.content,
    this.citations = const [],
    this.unknowns = const [],
    this.suggestedActions = const [],
    this.actionDetails = const [],
    this.receiptDetails = const [],
  });

  final String id;
  final String role;
  final String content;
  final List<AssistantCitation> citations;
  final List<String> unknowns;
  final List<String> suggestedActions;
  final List<AssistantSuggestedAction> actionDetails;
  final List<AssistantActionReceipt> receiptDetails;

  bool get isAssistant => role == 'assistant';

  factory AssistantTurn.fromJson(Map<String, dynamic> json) {
    final response = _map(json['response']);
    final assistantResponse = response.isEmpty
        ? null
        : AssistantResponse.fromJson(response);
    return AssistantTurn(
      id: _string(json['id'], 'message-unknown'),
      role: _string(json['role'], 'user'),
      content: _string(json['content'], assistantResponse?.answer ?? ''),
      citations: assistantResponse?.evidence ?? const [],
      unknowns: assistantResponse?.unknowns ?? const [],
      suggestedActions: assistantResponse?.suggestedActions ?? const [],
      actionDetails: assistantResponse?.actionDetails ?? const [],
      receiptDetails: assistantResponse?.receiptDetails ?? const [],
    );
  }
}

class WorkspaceConversationSummary {
  const WorkspaceConversationSummary({
    required this.id,
    required this.title,
    required this.status,
    required this.messageCount,
    this.context,
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String title;
  final String status;
  final int messageCount;
  final WorkspaceAssistantContext? context;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  factory WorkspaceConversationSummary.fromJson(Map<String, dynamic> json) {
    final context = _map(json['context']);
    return WorkspaceConversationSummary(
      id: _string(json['id'], 'conversation-unknown'),
      title: _string(json['title'], 'Untitled conversation'),
      status: _label(json['status']),
      messageCount: _int(json['messageCount'], 0),
      context: context.isEmpty
          ? null
          : WorkspaceAssistantContext.fromJson(context),
      createdAt: _date(json['createdAt']),
      updatedAt: _date(json['updatedAt']),
    );
  }
}

class WorkspaceConversation {
  const WorkspaceConversation({
    required this.id,
    required this.title,
    required this.status,
    required this.messages,
    this.context,
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String title;
  final String status;
  final List<AssistantTurn> messages;
  final WorkspaceAssistantContext? context;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  factory WorkspaceConversation.fromJson(Map<String, dynamic> json) {
    final context = _map(json['context']);
    return WorkspaceConversation(
      id: _string(json['id'], 'conversation-unknown'),
      title: _string(json['title'], 'Untitled conversation'),
      status: _label(json['status']),
      messages: _maps(
        json['messages'],
      ).map(AssistantTurn.fromJson).toList(growable: false),
      context: context.isEmpty
          ? null
          : WorkspaceAssistantContext.fromJson(context),
      createdAt: _date(json['createdAt']),
      updatedAt: _date(json['updatedAt']),
    );
  }
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
    this.capabilities = const <String, bool>{},
  });

  final String displayName;
  final WorkspaceDataOrigin origin;
  final List<WorkspaceProject> projects;
  final List<WorkspaceTask> tasks;
  final List<WorkspaceDecision> decisions;
  final List<KnowledgeSource> sources;
  final List<AssistantTurn> initialConversation;
  final Map<String, bool> capabilities;

  const WorkspaceData.empty({this.displayName = 'Developer'})
    : origin = WorkspaceDataOrigin.canonical,
      projects = const [],
      tasks = const [],
      decisions = const [],
      sources = const [],
      initialConversation = const [],
      capabilities = const <String, bool>{};

  WorkspaceProject projectFor(String id) => projects.firstWhere(
    (project) => project.id == id,
    orElse: () => const WorkspaceProject(
      id: '',
      name: 'Unassigned project',
      summary: 'This task is not linked to a project.',
      status: 'Unknown',
    ),
  );

  KnowledgeSource? sourceFor(String id) {
    for (final source in sources) {
      if (source.id == id) return source;
    }
    return null;
  }
}

class WorkspaceSnapshot {
  const WorkspaceSnapshot({
    required this.data,
    required this.conversationId,
    this.conversationContext,
    this.conversations = const [],
  });

  final WorkspaceData data;
  final String conversationId;
  final WorkspaceAssistantContext? conversationContext;
  final List<WorkspaceConversationSummary> conversations;

  factory WorkspaceSnapshot.fromJson(Map<String, dynamic> json) {
    final workspace = _map(json['workspace']);
    final conversation = _map(json['conversation']);
    final context = _map(conversation['context']);
    final messages = _maps(
      conversation['messages'],
    ).map(AssistantTurn.fromJson).toList(growable: false);
    return WorkspaceSnapshot(
      conversationId: _string(conversation['id'], ''),
      conversationContext: context.isEmpty
          ? null
          : WorkspaceAssistantContext.fromJson(context),
      conversations: _maps(
        json['conversations'],
      ).map(WorkspaceConversationSummary.fromJson).toList(growable: false),
      data: WorkspaceData(
        displayName: _string(workspace['name'], 'DevEnglish Workspace'),
        origin: WorkspaceDataOrigin.canonical,
        projects: _maps(
          json['projects'],
        ).map(WorkspaceProject.fromJson).toList(growable: false),
        tasks: _maps(
          json['tasks'],
        ).map(WorkspaceTask.fromJson).toList(growable: false),
        decisions: _maps(
          json['decisions'],
        ).map(WorkspaceDecision.fromJson).toList(growable: false),
        sources: _maps(
          json['sources'],
        ).map(KnowledgeSource.fromJson).toList(growable: false),
        initialConversation: messages,
        capabilities: _boolMap(json['capabilities']),
      ),
    );
  }
}

class WorkspaceAssistantResult {
  const WorkspaceAssistantResult({
    required this.conversationId,
    required this.response,
  });

  final String conversationId;
  final AssistantResponse response;

  factory WorkspaceAssistantResult.fromJson(Map<String, dynamic> json) =>
      WorkspaceAssistantResult(
        conversationId: _string(json['conversationId'], ''),
        response: AssistantResponse.fromJson(json),
      );
}

class ImportedWorkspaceSource {
  const ImportedWorkspaceSource({
    required this.source,
    required this.revisionId,
    required this.chunkId,
  });

  final KnowledgeSource source;
  final String revisionId;
  final String chunkId;

  factory ImportedWorkspaceSource.fromJson(Map<String, dynamic> json) {
    final source = _map(json['source']);
    return ImportedWorkspaceSource(
      source: KnowledgeSource.fromJson(source),
      revisionId: _string(json['revisionId'], ''),
      chunkId: _string(json['chunkId'], ''),
    );
  }
}

class WorkspaceSearchHit {
  const WorkspaceSearchHit({
    required this.id,
    required this.sourceId,
    required this.sourceName,
    required this.revisionId,
    required this.uri,
    required this.excerpt,
    required this.freshness,
    required this.origin,
    this.retrievalMode = '',
  });

  final String id;
  final String sourceId;
  final String sourceName;
  final String revisionId;
  final String uri;
  final String excerpt;
  final String freshness;
  final String origin;
  final String retrievalMode;

  factory WorkspaceSearchHit.fromJson(Map<String, dynamic> json) =>
      WorkspaceSearchHit(
        id: _string(json['id'], 'hit-unknown'),
        sourceId: _string(json['sourceId'], ''),
        sourceName: _string(json['sourceName'], 'Verified context'),
        revisionId: _string(json['revisionId'], ''),
        uri: _string(json['uri'], ''),
        excerpt: _string(json['excerpt'], ''),
        freshness: _label(json['freshness']),
        origin: _label(json['origin']),
        retrievalMode: _label(json['retrievalMode']),
      );
}

class WorkspaceSyncResult {
  const WorkspaceSyncResult({
    required this.seen,
    required this.upserted,
    required this.skipped,
    required this.nextCursor,
    required this.hasMore,
  });

  final int seen;
  final int upserted;
  final int skipped;
  final String nextCursor;
  final bool hasMore;

  factory WorkspaceSyncResult.fromJson(Map<String, dynamic> json) =>
      WorkspaceSyncResult(
        seen: _int(json['Seen'] ?? json['seen'], 0),
        upserted: _int(json['Upserted'] ?? json['upserted'], 0),
        skipped: _int(json['Skipped'] ?? json['skipped'], 0),
        nextCursor: _string(json['NextCursor'] ?? json['nextCursor'], ''),
        hasMore: json['HasMore'] == true || json['hasMore'] == true,
      );
}

class WorkspaceLearningObservation {
  const WorkspaceLearningObservation({
    required this.id,
    required this.sourceType,
    required this.sourceId,
    required this.skill,
    required this.prompt,
    required this.response,
    required this.feedback,
    required this.createdAt,
  });

  final String id;
  final String sourceType;
  final String sourceId;
  final String skill;
  final String prompt;
  final String response;
  final String feedback;
  final DateTime? createdAt;

  factory WorkspaceLearningObservation.fromJson(Map<String, dynamic> json) =>
      WorkspaceLearningObservation(
        id: _string(json['id'], 'observation-unknown'),
        sourceType: _string(json['sourceType'], ''),
        sourceId: _string(json['sourceId'], ''),
        skill: _string(json['skill'], 'learning'),
        prompt: _string(json['prompt'], ''),
        response: _string(json['response'], ''),
        feedback: _string(json['feedback'], ''),
        createdAt: _date(json['createdAt']),
      );
}

Map<String, dynamic> _map(dynamic value) =>
    value is Map ? Map<String, dynamic>.from(value) : <String, dynamic>{};

List<Map<String, dynamic>> _maps(dynamic value) => value is List
    ? value
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList()
    : <Map<String, dynamic>>[];

List<String> _strings(dynamic value) => value is List
    ? value.map((item) => item.toString()).toList(growable: false)
    : const [];

Map<String, bool> _boolMap(dynamic value) {
  if (value is! Map) return const <String, bool>{};
  return Map<String, bool>.fromEntries(
    value.entries
        .where((entry) => entry.key is String && entry.value is bool)
        .map((entry) => MapEntry(entry.key as String, entry.value as bool)),
  );
}

String _string(dynamic value, String fallback) {
  final result = value?.toString().trim() ?? '';
  return result.isEmpty ? fallback : result;
}

int _int(dynamic value, int fallback) {
  if (value is int) return value;
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

DateTime? _date(dynamic value) {
  final raw = value?.toString().trim() ?? '';
  if (raw.isEmpty) return null;
  return DateTime.tryParse(raw)?.toLocal();
}

String _label(dynamic value) {
  final raw = value?.toString().trim() ?? '';
  if (raw.isEmpty) return 'Unknown';
  return raw
      .split('_')
      .map(
        (word) => word.isEmpty
            ? word
            : '${word.substring(0, 1).toUpperCase()}${word.substring(1)}',
      )
      .join(' ');
}
