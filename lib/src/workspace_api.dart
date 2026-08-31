import 'api.dart';
import 'workspace_models.dart';

/// Production data boundary for the product-reset workspace.
///
/// This adapter only translates HTTP payloads into workspace models. Business
/// rules remain in the backend application service so the Flutter client and
/// future MCP transport cannot drift apart.
class WorkspaceApi {
  WorkspaceApi({DevEnglishApi? api}) : _api = api ?? DevEnglishApi();

  final DevEnglishApi _api;

  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      WorkspaceSnapshot.fromJson(
        await _api.workspaceBootstrap(includeTrashed: includeTrashed),
      );

  Future<WorkspaceProject> createProject({
    required String name,
    String description = '',
    required String idempotencyKey,
  }) async => WorkspaceProject.fromJson(
    await _api.workspaceCreateProject(
      name: name,
      description: description,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<WorkspaceTask> createTask({
    required String projectId,
    required String title,
    String description = '',
    String priority = 'normal',
    required String idempotencyKey,
  }) async => WorkspaceTask.fromJson(
    await _api.workspaceCreateTask(
      projectId: projectId,
      title: title,
      description: description,
      priority: priority,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<WorkspaceProject> updateProject({
    required String projectId,
    String? name,
    String? description,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => WorkspaceProject.fromJson(
    await _api.workspaceUpdateProject(
      projectId: projectId,
      name: name,
      description: description,
      status: status,
      expectedVersion: expectedVersion,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<WorkspaceTask> updateTask({
    required String taskId,
    String? projectId,
    String? title,
    String? description,
    String? status,
    String? priority,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => WorkspaceTask.fromJson(
    await _api.workspaceUpdateTask(
      taskId: taskId,
      projectId: projectId,
      title: title,
      description: description,
      status: status,
      priority: priority,
      expectedVersion: expectedVersion,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<WorkspaceDecision> createDecision({
    String projectId = '',
    required String title,
    String context = '',
    required String outcome,
    String rationale = '',
    String status = 'proposed',
    required String idempotencyKey,
  }) async => WorkspaceDecision.fromJson(
    await _api.workspaceCreateDecision(
      projectId: projectId,
      title: title,
      context: context,
      outcome: outcome,
      rationale: rationale,
      status: status,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<WorkspaceDecision> updateDecision({
    required String decisionId,
    String? projectId,
    String? title,
    String? context,
    String? outcome,
    String? rationale,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => WorkspaceDecision.fromJson(
    await _api.workspaceUpdateDecision(
      decisionId: decisionId,
      projectId: projectId,
      title: title,
      context: context,
      outcome: outcome,
      rationale: rationale,
      status: status,
      expectedVersion: expectedVersion,
      idempotencyKey: idempotencyKey,
    ),
  );

  Future<List<WorkspaceHistoryEvent>> projectHistory(String projectId) async {
    final payload = await _api.workspaceProjectHistory(projectId);
    return _history(payload);
  }

  Future<List<WorkspaceHistoryEvent>> taskHistory(String taskId) async {
    final payload = await _api.workspaceTaskHistory(taskId);
    return _history(payload);
  }

  Future<List<WorkspaceHistoryEvent>> decisionHistory(String decisionId) async {
    final payload = await _api.workspaceDecisionHistory(decisionId);
    return _history(payload);
  }

  Future<void> mutateState({
    required String path,
    required int expectedVersion,
    required String idempotencyKey,
  }) => _api
      .workspaceStateMutation(
        path: path,
        expectedVersion: expectedVersion,
        idempotencyKey: idempotencyKey,
      )
      .then((_) {});

  Future<ImportedWorkspaceSource> importManualSource({
    required String name,
    required String content,
    required String idempotencyKey,
    String kind = 'manual',
    String uri = '',
    String mimeType = 'text/plain',
  }) async => ImportedWorkspaceSource.fromJson(
    await _api.workspaceImportManualSource(
      name: name,
      content: content,
      idempotencyKey: idempotencyKey,
      kind: kind,
      uri: uri,
      mimeType: mimeType,
    ),
  );

  Future<List<WorkspaceSearchHit>> searchKnowledge(
    String query, {
    int limit = 20,
  }) async {
    final payload = await _api.workspaceSearchKnowledge(query, limit: limit);
    return _maps(
      payload['results'],
    ).map(WorkspaceSearchHit.fromJson).toList(growable: false);
  }

  Future<KnowledgeSourceDetail> knowledgeSourceDetail(String sourceId) async =>
      KnowledgeSourceDetail.fromJson(
        await _api.workspaceKnowledgeSourceDetail(sourceId),
      );

  Future<WorkspaceSyncResult> syncDrive({
    String cursor = '',
    int pageSize = 100,
  }) async => WorkspaceSyncResult.fromJson(
    await _api.workspaceSyncDrive(cursor: cursor, pageSize: pageSize),
  );

  Future<WorkspaceSyncResult> syncGitHub({
    required String repository,
    String cursor = '',
    int pageSize = 100,
  }) async => WorkspaceSyncResult.fromJson(
    await _api.workspaceSyncGitHub(
      repository: repository,
      cursor: cursor,
      pageSize: pageSize,
    ),
  );

  Future<WorkspaceActionChallenge> createActionChallenge(
    WorkspaceActionTarget target, {
    String idempotencyKey = '',
  }) async {
    final body = target.toJson();
    if (idempotencyKey.isNotEmpty) body['idempotencyKey'] = idempotencyKey;
    return WorkspaceActionChallenge.fromJson(
      await _api.workspaceCreateActionChallenge(body),
    );
  }

  Future<WorkspaceActionConfirmation> confirmAction(
    WorkspaceActionChallenge challenge, {
    String idempotencyKey = '',
  }) async {
    final body = challenge.target.toJson()
      ..['challengeId'] = challenge.action.challengeId
      ..['idempotencyKey'] = idempotencyKey.isEmpty
          ? challenge.action.idempotencyKey
          : idempotencyKey;
    return WorkspaceActionConfirmation.fromJson(
      await _api.workspaceConfirmAction(body),
    );
  }

  Future<void> recordLearningObservation({
    required String sourceType,
    String sourceId = '',
    required String skill,
    required String prompt,
    required String response,
    String feedback = '',
  }) async {
    await _api.workspaceRecordLearningObservation(
      sourceType: sourceType,
      sourceId: sourceId,
      skill: skill,
      prompt: prompt,
      response: response,
      feedback: feedback,
    );
  }

  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async {
    final payload = await _api.workspaceListLearningObservations(limit: limit);
    return _maps(
      payload['observations'],
    ).map(WorkspaceLearningObservation.fromJson).toList(growable: false);
  }

  Future<WorkspaceAssistantResult> startConversation(String message) async =>
      WorkspaceAssistantResult.fromJson(
        await _api.workspaceStartConversation(message),
      );

  Future<WorkspaceAssistantResult> startConversationWithContext(
    String message, {
    required WorkspaceAssistantContext context,
  }) async => WorkspaceAssistantResult.fromJson(
    await _api.workspaceStartConversation(
      message,
      context: context.toRequestJson(),
    ),
  );

  Future<WorkspaceAssistantResult> sendMessage(
    String conversationId,
    String message,
  ) async => WorkspaceAssistantResult.fromJson(
    await _api.workspaceSendMessage(conversationId, message),
  );

  Future<List<WorkspaceConversationSummary>> listConversations({
    int limit = 20,
  }) async {
    final payload = await _api.workspaceListConversations(limit: limit);
    return _maps(
      payload['conversations'],
    ).map(WorkspaceConversationSummary.fromJson).toList(growable: false);
  }

  Future<WorkspaceConversation> getConversation(String conversationId) async =>
      WorkspaceConversation.fromJson(
        await _api.workspaceGetConversation(conversationId),
      );
}

List<Map<String, dynamic>> _maps(dynamic value) => value is List
    ? value
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList()
    : <Map<String, dynamic>>[];

List<WorkspaceHistoryEvent> _history(Map<String, dynamic> payload) => _maps(
  payload['history'],
).map(WorkspaceHistoryEvent.fromJson).toList(growable: false);
