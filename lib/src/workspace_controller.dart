import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

import 'api.dart';
import 'workspace_demo.dart';
import 'workspace_models.dart';
import 'workspace_api.dart';

abstract interface class AssistantGateway {
  Future<AssistantResponse> ask(String prompt, WorkspaceData data);
}

class DemoAssistantGateway implements AssistantGateway {
  const DemoAssistantGateway();

  @override
  Future<AssistantResponse> ask(String prompt, WorkspaceData data) async {
    await Future<void>.delayed(const Duration(milliseconds: 120));
    final lower = prompt.toLowerCase();

    if (lower.contains('unknown') || lower.contains('không có')) {
      return _unknownResponse();
    }

    if (lower.contains('task') ||
        lower.contains('việc') ||
        lower.contains('tiến độ')) {
      final citation = _citationFor(data, 'evidence-wave3-task');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'The current high-priority task is to finish the Today, Work and Knowledge workspace UI. The next safe step is to validate the text assistant flow and its citations before adding voice features.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [
          'Review the UI flow',
          'Check citation visibility',
        ],
        actionReceipts: const [],
      );
    }

    if (lower.contains('today') || lower.contains('workspace')) {
      final citation = _citationFor(data, 'evidence-wave3-ui');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'Today is the default workspace, with Work, Knowledge and Learning available as separate destinations.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [],
        actionReceipts: const [],
      );
    }

    if (lower.contains('mcp') || lower.contains('rest')) {
      final citation = _citationFor(data, 'evidence-wave3-mcp');
      if (citation == null) return _unknownResponse();
      return AssistantResponse(
        answer:
            'MCP read tools reuse the same application-service semantics as REST, while write tools stay behind explicit confirmation.',
        evidence: [citation],
        unknowns: const [],
        staleSources: const [],
        suggestedActions: const [],
        actionReceipts: const [],
      );
    }

    return _unknownResponse();
  }

  AssistantResponse _unknownResponse() => const AssistantResponse(
    answer:
        'I cannot verify that from the connected project sources yet. I will keep it as unknown instead of inventing a project fact.',
    evidence: [],
    unknowns: ['The requested detail is not present in the preview source.'],
    staleSources: [],
    suggestedActions: [
      'Add a source or record a decision before relying on this detail.',
    ],
    actionReceipts: [],
  );

  AssistantCitation? _citationFor(WorkspaceData data, String evidenceId) {
    for (final source in data.sources) {
      for (final evidence in source.evidence) {
        if (evidence.id == evidenceId) {
          return AssistantCitation(
            sourceId: source.id,
            sourceTitle: source.title,
            location: evidence.location,
            excerpt: evidence.excerpt,
            origin: data.origin,
          );
        }
      }
    }
    return null;
  }
}

class WorkspaceController extends ChangeNotifier {
  WorkspaceController({
    WorkspaceData? data,
    AssistantGateway? assistantGateway,
    WorkspaceApi? workspaceApi,
  }) : _data = data ?? WorkspaceDemo.data,
       _assistantGateway = assistantGateway ?? const DemoAssistantGateway(),
       _workspaceApi = workspaceApi,
       _conversation = List<AssistantTurn>.of(
         (data ?? WorkspaceDemo.data).initialConversation,
       ),
       _canonicalLoaded =
           workspaceApi == null &&
           data?.origin == WorkspaceDataOrigin.canonical;

  WorkspaceData _data;
  final AssistantGateway _assistantGateway;
  final WorkspaceApi? _workspaceApi;
  final List<AssistantTurn> _conversation;
  final List<WorkspaceConversationSummary> _conversationHistory = [];
  String _conversationId = '';
  bool _loading = false;
  bool _canonicalLoaded;
  bool _sending = false;
  bool _conversationLoading = false;
  bool _includeTrashed = false;
  String? _error;
  List<WorkspaceLearningObservation> _learningObservations = const [];
  bool _actionBusy = false;
  AssistantActionReceipt? _lastActionReceipt;
  WorkspaceAssistantContext? _assistantContext;
  int _sessionGeneration = 0;

  WorkspaceData get data => _data;
  List<AssistantTurn> get conversation => List.unmodifiable(_conversation);
  List<WorkspaceConversationSummary> get conversationHistory =>
      List.unmodifiable(_conversationHistory);
  String get conversationId => _conversationId;
  bool get loading => _loading;
  bool get hasWorkspaceApi => _workspaceApi != null;
  bool get canonicalLoaded => _canonicalLoaded;
  bool get usingDemo => _data.origin.isPreview;
  bool get sending => _sending;
  bool get conversationLoading => _conversationLoading;
  String? get error => _error;
  bool get includeTrashed => _includeTrashed;
  bool get actionBusy => _actionBusy;
  AssistantActionReceipt? get lastActionReceipt => _lastActionReceipt;
  WorkspaceAssistantContext? get assistantContext => _assistantContext;
  List<WorkspaceLearningObservation> get learningObservations =>
      List.unmodifiable(_learningObservations);

  /// Clears user-scoped workspace state after logout. A subsequent login must
  /// bootstrap the new principal instead of displaying the previous user's
  /// projects, sources or conversation history from this process.
  void resetForLogout() {
    _sessionGeneration++;
    _data = _workspaceApi == null
        ? WorkspaceDemo.data
        : const WorkspaceData.empty();
    _conversation.clear();
    _conversationHistory.clear();
    _conversationId = '';
    _assistantContext = null;
    _learningObservations = const [];
    _lastActionReceipt = null;
    _error = null;
    _loading = false;
    _sending = false;
    _conversationLoading = false;
    _actionBusy = false;
    _canonicalLoaded =
        _workspaceApi == null && _data.origin == WorkspaceDataOrigin.canonical;
    notifyListeners();
  }

  Future<void> load({bool? includeTrashed}) async {
    if (includeTrashed != null) _includeTrashed = includeTrashed;
    if (_workspaceApi == null || _loading) return;
    final generation = _sessionGeneration;
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final snapshot = await _workspaceApi.bootstrap(
        includeTrashed: _includeTrashed,
      );
      if (generation != _sessionGeneration) return;
      _data = snapshot.data;
      _conversationId = snapshot.conversationId;
      _assistantContext = snapshot.conversationContext;
      _conversationHistory
        ..clear()
        ..addAll(snapshot.conversations);
      _conversation
        ..clear()
        ..addAll(snapshot.data.initialConversation);
      try {
        _learningObservations = await _workspaceApi.listLearningObservations();
      } catch (_) {
        if (generation != _sessionGeneration) return;
        // Learning history is an enhancement to the workspace snapshot. A
        // read failure must not hide otherwise usable canonical work data.
        _learningObservations = const [];
      }
      if (generation != _sessionGeneration) return;
      _canonicalLoaded = true;
    } catch (error) {
      if (generation != _sessionGeneration) return;
      _error = _friendlyError(
        error,
        'Không thể tải workspace thật từ backend. Hãy kiểm tra server và thử lại.',
      );
      _canonicalLoaded = false;
    } finally {
      if (generation == _sessionGeneration) {
        _loading = false;
        notifyListeners();
      }
    }
  }

  Future<void> ask(String prompt, {WorkspaceAssistantContext? context}) async {
    final trimmed = prompt.trim();
    if (trimmed.isEmpty || _sending) return;
    final generation = _sessionGeneration;

    if (_workspaceApi != null &&
        context != null &&
        context != _assistantContext &&
        _conversationId.isNotEmpty) {
      _conversationId = '';
      _assistantContext = null;
      _conversation.clear();
    }

    _conversation.add(
      AssistantTurn(
        id: 'user-${_conversation.length}',
        role: 'user',
        content: trimmed,
      ),
    );
    _sending = true;
    _error = null;
    notifyListeners();

    try {
      late AssistantResponse response;
      if (_workspaceApi == null) {
        response = await _assistantGateway.ask(trimmed, data);
      } else {
        final result = _conversationId.isEmpty
            ? context == null
                  ? await _workspaceApi.startConversation(trimmed)
                  : await _workspaceApi.startConversationWithContext(
                      trimmed,
                      context: context,
                    )
            : await _workspaceApi.sendMessage(_conversationId, trimmed);
        if (!_isCurrent(generation)) return;
        _conversationId = result.conversationId;
        if (context != null) _assistantContext = context;
        response = result.response;
        await _refreshConversationHistory(generation: generation);
      }
      if (!_isCurrent(generation)) return;
      _conversation.add(
        AssistantTurn(
          id: 'assistant-${_conversation.length}',
          role: 'assistant',
          content: response.answer,
          citations: response.evidence,
          unknowns: response.unknowns,
          suggestedActions: response.suggestedActions,
          actionDetails: response.actionDetails,
          receiptDetails: response.receiptDetails,
        ),
      );
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'The assistant is unavailable. Your message is still visible in this session.',
      );
    } finally {
      if (_isCurrent(generation)) {
        _sending = false;
        notifyListeners();
      }
    }
  }

  Future<void> openConversation(String conversationId) async {
    final api = _workspaceApi;
    final id = conversationId.trim();
    if (api == null || id.isEmpty || _conversationLoading || _sending) return;
    final generation = _sessionGeneration;
    _conversationLoading = true;
    _error = null;
    notifyListeners();
    try {
      final conversation = await api.getConversation(id);
      if (!_isCurrent(generation)) return;
      _conversationId = conversation.id;
      _assistantContext = conversation.context;
      _conversation
        ..clear()
        ..addAll(conversation.messages);
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể mở lại cuộc trò chuyện. Hãy thử lại.',
      );
    } finally {
      if (_isCurrent(generation)) {
        _conversationLoading = false;
        notifyListeners();
      }
    }
  }

  void startNewConversation() {
    if (_sending || _conversationLoading) return;
    _conversationId = '';
    _assistantContext = null;
    _conversation.clear();
    _error = null;
    notifyListeners();
  }

  Future<void> _refreshConversationHistory({int? generation}) async {
    final api = _workspaceApi;
    if (api == null) return;
    final expectedGeneration = generation ?? _sessionGeneration;
    try {
      final items = await api.listConversations();
      if (!_isCurrent(expectedGeneration)) return;
      _conversationHistory
        ..clear()
        ..addAll(items);
      notifyListeners();
    } catch (_) {
      // Conversation history is navigation metadata; a failure must not hide
      // the response that was already persisted and rendered.
    }
  }

  Future<void> createProject({
    required String name,
    String description = '',
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.createProject(
        name: name,
        description: description,
        idempotencyKey: _mutationKey('project'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể tạo project. Dữ liệu chưa được ghi.',
      );
      notifyListeners();
    }
  }

  Future<void> createTask({
    required String projectId,
    required String title,
    String description = '',
    String priority = 'normal',
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.createTask(
        projectId: projectId,
        title: title,
        description: description,
        priority: priority,
        idempotencyKey: _mutationKey('task'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể tạo task. Dữ liệu chưa được ghi.',
      );
      notifyListeners();
    }
  }

  Future<void> updateProject({
    required WorkspaceProject project,
    String? name,
    String? description,
    String? status,
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.updateProject(
        projectId: project.id,
        name: name,
        description: description,
        status: status,
        expectedVersion: project.version,
        idempotencyKey: _mutationKey('project-update'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể cập nhật project. Có thể dữ liệu đã đổi; hãy tải lại rồi thử lại.',
      );
      notifyListeners();
    }
  }

  Future<void> updateTask({
    required WorkspaceTask task,
    String? title,
    String? description,
    String? status,
    String? priority,
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.updateTask(
        taskId: task.id,
        title: title,
        description: description,
        status: status,
        priority: priority,
        expectedVersion: task.version,
        idempotencyKey: _mutationKey('task-update'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể cập nhật task. Có thể dữ liệu đã đổi; hãy tải lại rồi thử lại.',
      );
      notifyListeners();
    }
  }

  Future<void> createDecision({
    String projectId = '',
    required String title,
    String context = '',
    required String outcome,
    String rationale = '',
    String status = 'proposed',
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.createDecision(
        projectId: projectId,
        title: title,
        context: context,
        outcome: outcome,
        rationale: rationale,
        status: status,
        idempotencyKey: _mutationKey('decision'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể ghi decision. Dữ liệu chưa được ghi.',
      );
      notifyListeners();
    }
  }

  Future<void> updateDecision({
    required WorkspaceDecision decision,
    String? projectId,
    String? title,
    String? context,
    String? outcome,
    String? rationale,
    String? status,
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.updateDecision(
        decisionId: decision.id,
        projectId: projectId,
        title: title,
        context: context,
        outcome: outcome,
        rationale: rationale,
        status: status,
        expectedVersion: decision.version,
        idempotencyKey: _mutationKey('decision-update'),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể cập nhật decision. Có thể dữ liệu đã đổi; hãy tải lại rồi thử lại.',
      );
      notifyListeners();
    }
  }

  Future<void> trashProject(WorkspaceProject project) => _trashWithAction(
    entityType: 'project',
    entityId: project.id,
    expectedVersion: project.version,
    error: 'Không thể đưa project vào thùng rác.',
  );

  Future<void> restoreProject(WorkspaceProject project) => _mutateState(
    path: '/api/v2/projects/${project.id}/restore',
    expectedVersion: project.version,
    prefix: 'project-restore',
    error: 'Không thể khôi phục project.',
  );

  Future<void> trashTask(WorkspaceTask task) => _trashWithAction(
    entityType: 'task',
    entityId: task.id,
    expectedVersion: task.version,
    error: 'Không thể đưa task vào thùng rác.',
  );

  Future<void> restoreTask(WorkspaceTask task) => _mutateState(
    path: '/api/v2/tasks/${task.id}/restore',
    expectedVersion: task.version,
    prefix: 'task-restore',
    error: 'Không thể khôi phục task.',
  );

  Future<void> trashDecision(WorkspaceDecision decision) => _trashWithAction(
    entityType: 'decision',
    entityId: decision.id,
    expectedVersion: decision.version,
    error: 'Không thể đưa decision vào thùng rác.',
  );

  Future<void> restoreDecision(WorkspaceDecision decision) => _mutateState(
    path: '/api/v2/decisions/${decision.id}/restore',
    expectedVersion: decision.version,
    prefix: 'decision-restore',
    error: 'Không thể khôi phục decision.',
  );

  Future<void> purgeProject(
    WorkspaceProject project,
  ) => _workActionWithChallenge(
    operation: 'work.entity.purge',
    entityType: 'project',
    entityId: project.id,
    expectedVersion: project.version,
    error:
        'Không thể xoá vĩnh viễn project. Chỉ bản ghi đã đủ thời gian lưu trữ mới được purge.',
  );

  Future<void> purgeTask(WorkspaceTask task) => _workActionWithChallenge(
    operation: 'work.entity.purge',
    entityType: 'task',
    entityId: task.id,
    expectedVersion: task.version,
    error:
        'Không thể xoá vĩnh viễn task. Chỉ bản ghi đã đủ thời gian lưu trữ mới được purge.',
  );

  Future<void> purgeDecision(
    WorkspaceDecision decision,
  ) => _workActionWithChallenge(
    operation: 'work.entity.purge',
    entityType: 'decision',
    entityId: decision.id,
    expectedVersion: decision.version,
    error:
        'Không thể xoá vĩnh viễn decision. Chỉ bản ghi đã đủ thời gian lưu trữ mới được purge.',
  );

  Future<void> _mutateState({
    required String path,
    required int expectedVersion,
    required String prefix,
    required String error,
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.mutateState(
        path: path,
        expectedVersion: expectedVersion,
        idempotencyKey: _mutationKey(prefix),
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (failure) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(failure, error);
      notifyListeners();
    }
  }

  Future<void> _trashWithAction({
    required String entityType,
    required String entityId,
    required int expectedVersion,
    required String error,
  }) => _workActionWithChallenge(
    operation: 'work.entity.trash',
    entityType: entityType,
    entityId: entityId,
    expectedVersion: expectedVersion,
    error: error,
  );

  Future<void> _workActionWithChallenge({
    required String operation,
    required String entityType,
    required String entityId,
    required int expectedVersion,
    required String error,
  }) async {
    final generation = _sessionGeneration;
    final challenge = await createActionChallenge(
      WorkspaceActionTarget(
        operation: operation,
        entityType: entityType,
        entityId: entityId,
        expectedVersion: expectedVersion,
      ),
    );
    if (!_isCurrent(generation) || challenge == null) return;
    final confirmation = await confirmAction(challenge);
    if (!_isCurrent(generation) || confirmation == null) return;
    try {
      await load();
    } catch (failure) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(failure, error);
      notifyListeners();
    }
  }

  Future<void> importManualSource({
    required String name,
    required String content,
    String kind = 'manual',
    String uri = '',
  }) async {
    final api = _workspaceApi;
    if (api == null) return;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.importManualSource(
        name: name,
        content: content,
        idempotencyKey: _mutationKey('knowledge-import'),
        kind: kind,
        uri: uri,
      );
      if (!_isCurrent(generation)) return;
      await load();
    } catch (error) {
      if (!_isCurrent(generation)) return;
      _error = _friendlyError(
        error,
        'Không thể nhập knowledge source. Dữ liệu chưa được ghi.',
      );
      notifyListeners();
    }
  }

  Future<List<WorkspaceSearchHit>> searchKnowledge(String query) async {
    final api = _workspaceApi;
    if (api == null || query.trim().isEmpty) return const [];
    final generation = _sessionGeneration;
    try {
      final result = await api.searchKnowledge(query);
      return _isCurrent(generation) ? result : const [];
    } catch (error) {
      if (!_isCurrent(generation)) return const [];
      _error = _friendlyError(error, 'Không thể tìm trong knowledge lúc này.');
      notifyListeners();
      return const [];
    }
  }

  Future<KnowledgeSourceDetail?> knowledgeSourceDetail(String sourceId) async {
    final api = _workspaceApi;
    if (api == null || sourceId.trim().isEmpty) return null;
    final generation = _sessionGeneration;
    try {
      final result = await api.knowledgeSourceDetail(sourceId.trim());
      return _isCurrent(generation) ? result : null;
    } catch (error) {
      if (!_isCurrent(generation)) return null;
      _error = _friendlyError(
        error,
        'Không thể tải lịch sử của knowledge source lúc này.',
      );
      notifyListeners();
      return null;
    }
  }

  Future<List<WorkspaceHistoryEvent>> projectHistory(String projectId) {
    final api = _workspaceApi;
    if (api == null || projectId.trim().isEmpty) return Future.value(const []);
    final generation = _sessionGeneration;
    return _loadHistory(
      () => api.projectHistory(projectId.trim()),
      'Không thể tải lịch sử project lúc này.',
      generation: generation,
    );
  }

  Future<List<WorkspaceHistoryEvent>> taskHistory(String taskId) {
    final api = _workspaceApi;
    if (api == null || taskId.trim().isEmpty) return Future.value(const []);
    final generation = _sessionGeneration;
    return _loadHistory(
      () => api.taskHistory(taskId.trim()),
      'Không thể tải lịch sử task lúc này.',
      generation: generation,
    );
  }

  Future<List<WorkspaceHistoryEvent>> decisionHistory(String decisionId) {
    final api = _workspaceApi;
    if (api == null || decisionId.trim().isEmpty) return Future.value(const []);
    final generation = _sessionGeneration;
    return _loadHistory(
      () => api.decisionHistory(decisionId.trim()),
      'Không thể tải lịch sử decision lúc này.',
      generation: generation,
    );
  }

  Future<List<WorkspaceHistoryEvent>> _loadHistory(
    Future<List<WorkspaceHistoryEvent>> Function() request,
    String fallback, {
    required int generation,
  }) async {
    try {
      final result = await request();
      return _isCurrent(generation) ? result : const [];
    } catch (error) {
      if (!_isCurrent(generation)) return const [];
      _error = _friendlyError(error, fallback);
      notifyListeners();
      return const [];
    }
  }

  Future<WorkspaceSyncResult?> syncDrive() async {
    final api = _workspaceApi;
    if (api == null) return null;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      final result = await api.syncDrive();
      if (!_isCurrent(generation)) return null;
      await load();
      if (!_isCurrent(generation)) return null;
      return result;
    } catch (error) {
      if (!_isCurrent(generation)) return null;
      _error = _friendlyError(error, 'Không thể đồng bộ Google Drive lúc này.');
      notifyListeners();
      return null;
    }
  }

  Future<WorkspaceSyncResult?> syncGitHub(String repository) async {
    final api = _workspaceApi;
    if (api == null || repository.trim().isEmpty) return null;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      final result = await api.syncGitHub(repository: repository.trim());
      if (!_isCurrent(generation)) return null;
      await load();
      if (!_isCurrent(generation)) return null;
      return result;
    } catch (error) {
      if (!_isCurrent(generation)) return null;
      _error = _friendlyError(
        error,
        'Không thể đồng bộ GitHub repository lúc này.',
      );
      notifyListeners();
      return null;
    }
  }

  Future<bool> recordLearningObservation({
    required String sourceType,
    String sourceId = '',
    required String skill,
    required String prompt,
    required String response,
    String feedback = '',
  }) async {
    final api = _workspaceApi;
    if (api == null || response.trim().isEmpty) return false;
    final generation = _sessionGeneration;
    _error = null;
    notifyListeners();
    try {
      await api.recordLearningObservation(
        sourceType: sourceType,
        sourceId: sourceId,
        skill: skill,
        prompt: prompt,
        response: response,
        feedback: feedback,
      );
      if (!_isCurrent(generation)) return false;
      try {
        _learningObservations = await api.listLearningObservations();
      } catch (_) {
        if (!_isCurrent(generation)) return false;
        // Saving the observation succeeded; keep the local screen usable if
        // the follow-up history refresh is unavailable.
      }
      if (!_isCurrent(generation)) return false;
      notifyListeners();
      return true;
    } catch (error) {
      if (!_isCurrent(generation)) return false;
      _error = _friendlyError(
        error,
        'Không thể lưu learning note lúc này. Nội dung vẫn còn trên màn hình.',
      );
      notifyListeners();
      return false;
    }
  }

  Future<WorkspaceActionChallenge?> createActionChallenge(
    WorkspaceActionTarget target,
  ) async {
    final api = _workspaceApi;
    if (api == null || !_validActionTarget(target) || _actionBusy) {
      return null;
    }
    final generation = _sessionGeneration;
    _actionBusy = true;
    _error = null;
    notifyListeners();
    try {
      final challenge = await api.createActionChallenge(
        target,
        idempotencyKey: _mutationKey('action-challenge'),
      );
      return _isCurrent(generation) ? challenge : null;
    } catch (error) {
      if (!_isCurrent(generation)) return null;
      _error = _friendlyError(
        error,
        'Không thể tạo preview action. Chưa có thay đổi nào được thực thi.',
      );
      notifyListeners();
      return null;
    } finally {
      if (_isCurrent(generation)) {
        _actionBusy = false;
        notifyListeners();
      }
    }
  }

  bool _validActionTarget(WorkspaceActionTarget target) {
    if (target.operation == 'work.entity.trash' ||
        target.operation == 'work.entity.purge') {
      return (target.entityType == 'project' ||
              target.entityType == 'task' ||
              target.entityType == 'decision') &&
          target.entityId.trim().isNotEmpty &&
          target.expectedVersion > 0 &&
          target.repository.trim().isEmpty &&
          target.issue == 0 &&
          target.title.isEmpty &&
          target.body.isEmpty &&
          target.labels.isEmpty &&
          target.expectedRevision.isEmpty;
    }
    return target.repository.trim().isNotEmpty &&
        target.entityType.isEmpty &&
        target.entityId.isEmpty &&
        target.expectedVersion == 0;
  }

  Future<WorkspaceActionConfirmation?> confirmAction(
    WorkspaceActionChallenge challenge,
  ) async {
    final api = _workspaceApi;
    if (api == null || _actionBusy) return null;
    final generation = _sessionGeneration;
    _actionBusy = true;
    _error = null;
    notifyListeners();
    try {
      final confirmation = await api.confirmAction(challenge);
      if (!_isCurrent(generation)) return null;
      _lastActionReceipt = confirmation.receipt;
      notifyListeners();
      return confirmation;
    } catch (error) {
      if (!_isCurrent(generation)) return null;
      _error = _friendlyError(
        error,
        'Action chưa được thực thi. Challenge có thể đã hết hạn hoặc không còn khớp.',
      );
      notifyListeners();
      return null;
    } finally {
      if (_isCurrent(generation)) {
        _actionBusy = false;
        notifyListeners();
      }
    }
  }

  String _friendlyError(Object error, String fallback) {
    if (error is TimeoutException || error is http.ClientException) {
      return 'Không thể kết nối backend. Kiểm tra mạng hoặc server rồi thử lại.';
    }
    if (error is ApiException) {
      if (error.statusCode == 0) {
        return 'Không thể kết nối backend. Kiểm tra mạng hoặc server rồi thử lại.';
      }
      if (error.isConflict) {
        return 'Dữ liệu đã thay đổi ở nơi khác. Hãy tải lại workspace rồi thử lại.';
      }
      if (error.statusCode == 401 || error.statusCode == 403) {
        return 'Phiên đăng nhập hoặc quyền truy cập không còn hợp lệ.';
      }
      if (error.statusCode == 429) {
        return 'Tác vụ đang tạm vượt hạn mức. Hãy thử lại sau.';
      }
      if (error.statusCode >= 500) {
        return 'Backend đang gặp sự cố tạm thời. Hãy thử lại sau.';
      }
    }
    return fallback;
  }

  String _mutationKey(String prefix) =>
      '$prefix-${DateTime.now().microsecondsSinceEpoch}';

  bool _isCurrent(int generation) => generation == _sessionGeneration;
}
