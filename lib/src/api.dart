import 'dart:convert';

import 'package:http/http.dart' as http;

import 'http_client.dart';
import 'models.dart';

class AuthRequiredException implements Exception {
  const AuthRequiredException([this.message = 'Authentication is required.']);

  final String message;

  @override
  String toString() => message;
}

/// A safe, structured API failure that the UI can use for conflict and quota
/// handling without parsing provider-specific error strings.
class ApiException implements Exception {
  const ApiException(this.statusCode, this.message, {this.code});

  final int statusCode;
  final String message;
  final String? code;

  bool get isConflict => statusCode == 409;

  @override
  String toString() =>
      'ApiException($statusCode${code == null ? '' : ', $code'}): $message';
}

class DevEnglishApi {
  static const defaultWorkContextTimeout = Duration(seconds: 90);

  DevEnglishApi({
    http.Client? client,
    String? baseUrl,
    Duration? workContextTimeout,
  }) : _client = client ?? createHttpClient(),
       _workContextTimeout = workContextTimeout ?? defaultWorkContextTimeout,
       baseUrl =
           baseUrl ??
           const String.fromEnvironment(
             'API_BASE_URL',
             defaultValue: 'http://localhost:8080',
           );

  final http.Client _client;
  final Duration _workContextTimeout;
  final String baseUrl;
  void Function()? onUnauthorized;

  Future<AuthUser> currentUser() async =>
      AuthUser.fromJson(await _get('/api/v1/auth/me'));

  Future<AuthUser> login(String secret) async {
    final json = await _post('/api/v1/auth/login', {'secret': secret});
    return AuthUser.fromJson(_map(json['user']));
  }

  Future<void> logout() async {
    await _post('/api/v1/auth/logout', <String, dynamic>{});
  }

  Future<HomeData> home() async => HomeData.fromJson(
    await _get('/api/v1/home', timeout: const Duration(seconds: 30)),
  );

  Future<SettingsData> settings() async =>
      SettingsData.fromJson(await _get('/api/v1/settings'));

  Future<SettingsData> updateSettings(SettingsData settings) async =>
      SettingsData.fromJson(
        await _put('/api/v1/settings', {
          'aiProvider': settings.aiProvider,
          'fastModel': settings.fastModel,
          'smartModel': settings.smartModel,
          'pronunciationOn': settings.pronunciationOn,
          'monthlyBudgetVnd': settings.monthlyBudgetVnd,
        }),
      );

  Future<SettingsData> setDeepSeekKey(String apiKey) async =>
      SettingsData.fromJson(
        _map(
          (await _put('/api/v1/settings/deepseek', {
            'apiKey': apiKey,
          }))['settings'],
        ),
      );

  Future<SettingsData> testDeepSeekKey() async => SettingsData.fromJson(
    _map((await _post('/api/v1/settings/deepseek/test', {}))['settings']),
  );

  Future<SettingsData> removeDeepSeekKey() async =>
      SettingsData.fromJson(await _delete('/api/v1/settings/deepseek'));

  Future<List<PracticeMode>> practice() async {
    final json = await _get('/api/v1/practice');
    return _list(json['modes']).map(PracticeMode.fromJson).toList();
  }

  Future<List<ReviewItem>> reviewDue() async {
    final json = await _get('/api/v1/review/due');
    return _list(json['items']).map(ReviewItem.fromJson).toList();
  }

  Future<ProgressData> progress() async =>
      ProgressData.fromJson(await _get('/api/v1/progress'));

  Future<List<DiagnosticQuestion>> diagnosticQuestions() async {
    final json = await _get('/api/v1/diagnostic/questions');
    return _list(json['questions']).map(DiagnosticQuestion.fromJson).toList();
  }

  Future<DiagnosticResult> diagnosticResult() async =>
      DiagnosticResult.fromJson(await _get('/api/v1/diagnostic'));

  Future<DiagnosticResult> submitDiagnostic(
    List<Map<String, String>> responses,
  ) async => DiagnosticResult.fromJson(
    await _post('/api/v1/diagnostic', {'responses': responses}),
  );

  Future<List<RoleplayScenario>> roleplayScenarios() async {
    final json = await _get('/api/v1/roleplay/scenarios');
    return _list(json['scenarios']).map(RoleplayScenario.fromJson).toList();
  }

  Future<Conversation> startRoleplay(String scenarioId) async =>
      Conversation.fromJson(
        await _post('/api/v1/roleplay/conversations', {
          'scenarioId': scenarioId,
        }),
      );

  Future<RoleplayTurnResult> roleplayTurn(
    String conversationId,
    String answer,
  ) async => RoleplayTurnResult.fromJson(
    await _post(
      '/api/v1/roleplay/conversations/$conversationId/turns',
      {'answer': answer},
      timeout: const Duration(seconds: 90),
    ),
  );

  Future<CopilotResult> copilot({
    required String vietnamese,
    String context = '',
  }) async => CopilotResult.fromJson(
    await _post('/api/v1/copilot', {
      'vietnamese': vietnamese,
      'context': context,
    }),
  );

  Future<List<VocabularyItem>> vocabulary() async {
    final json = await _get('/api/v1/vocabulary');
    return _list(json['items']).map(VocabularyItem.fromJson).toList();
  }

  Future<VocabularyGraph> vocabularyGraph() async =>
      VocabularyGraph.fromJson(await _get('/api/v1/vocabulary/graph'));

  Future<AnalyticsSummary> analytics() async =>
      AnalyticsSummary.fromJson(await _get('/api/v1/analytics'));

  Future<WeeklySpeakingAssessment> weeklySpeaking() async =>
      WeeklySpeakingAssessment.fromJson(await _get('/api/v1/speaking/weekly'));

  Future<List<ProviderCheck>> testConnections() async {
    final json = await _get('/api/v1/settings/test');
    return _list(json['providers']).map(ProviderCheck.fromJson).toList();
  }

  Future<UsageSummary> usage() async =>
      UsageSummary.fromJson(await _get('/api/v1/usage'));

  Future<List<int>> synthesize({
    required String text,
    String voice = '',
  }) async {
    final response = await _client
        .post(
          Uri.parse('$baseUrl/api/v1/speaking/synthesize'),
          headers: _headers({'content-type': 'application/json'}),
          body: jsonEncode({'text': text, 'voice': voice}),
        )
        .timeout(const Duration(seconds: 30));
    if (response.statusCode < 200 || response.statusCode >= 300) {
      if (response.statusCode == 401) {
        onUnauthorized?.call();
        throw const AuthRequiredException('Authentication is required.');
      }
      throw Exception('TTS request failed');
    }
    return response.bodyBytes;
  }

  Future<void> submitReview({
    required String kind,
    required String id,
    required bool success,
    required double score,
  }) async {
    await _post('/api/v1/review/$kind/$id', {
      'success': success,
      'score': score,
    });
  }

  Future<SpeakingSession> transcribeAudio({
    required List<int> bytes,
    required String mimeType,
    String missionId = '',
  }) async => SpeakingSession.fromJson(
    await _multipart('/api/v1/speaking/transcribe', bytes, mimeType, {
      'missionId': missionId,
    }),
  );

  Future<SpeakingSession> saveTranscript({
    required String transcript,
    String missionId = '',
  }) async => SpeakingSession.fromJson(
    await _post('/api/v1/speaking/transcript', {
      'missionId': missionId,
      'transcript': transcript,
    }),
  );

  Future<SpeakingSession> assessSpeaking({
    required List<int> bytes,
    required String mimeType,
    required String sessionId,
    String reference = '',
  }) async => SpeakingSession.fromJson(
    await _multipart('/api/v1/speaking/assess', bytes, mimeType, {
      'sessionId': sessionId,
      'reference': reference,
    }),
  );

  Future<SubmissionResult> submitWriting(
    String missionId,
    String answer,
  ) async => SubmissionResult.fromJson(
    await _post('/api/v1/missions/$missionId/attempts', {
      'answer': answer,
    }, timeout: const Duration(seconds: 90)),
  );

  Future<WorkImportResult> importWork({
    required String sourceType,
    required String title,
    required String content,
  }) async => WorkImportResult.fromJson(
    await _post('/api/v1/work-context', {
      'sourceType': sourceType,
      'title': title,
      'content': content,
    }, timeout: _workContextTimeout),
  );

  Future<WorkImportResult> importGitHub(String url) async =>
      WorkImportResult.fromJson(
        await _post('/api/v1/integrations/github/import', {'url': url}),
      );

  // Product-reset endpoints intentionally live under v2 so the legacy
  // learning contract can remain compatible during the cutover.
  Future<Map<String, dynamic>> workspaceBootstrap({
    bool includeTrashed = false,
  }) async => _get(
    '/api/v2/bootstrap?includeTrashed=$includeTrashed',
    timeout: const Duration(seconds: 15),
  );

  Future<Map<String, dynamic>> workspaceCreateProject({
    required String name,
    String description = '',
    required String idempotencyKey,
  }) async => _post(
    '/api/v2/projects',
    {'name': name, 'description': description},
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceCreateTask({
    required String projectId,
    required String title,
    String description = '',
    String priority = 'normal',
    required String idempotencyKey,
  }) async => _post(
    '/api/v2/projects/$projectId/tasks',
    {'title': title, 'description': description, 'priority': priority},
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceUpdateProject({
    required String projectId,
    String? name,
    String? description,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => _patch(
    '/api/v2/projects/$projectId',
    {
      'name': ?name,
      'description': ?description,
      'status': ?status,
      'expectedVersion': expectedVersion,
    },
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceUpdateTask({
    required String taskId,
    String? projectId,
    String? title,
    String? description,
    String? status,
    String? priority,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => _patch(
    '/api/v2/tasks/$taskId',
    {
      'projectId': ?projectId,
      'title': ?title,
      'description': ?description,
      'status': ?status,
      'priority': ?priority,
      'expectedVersion': expectedVersion,
    },
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceCreateDecision({
    String projectId = '',
    required String title,
    String context = '',
    required String outcome,
    String rationale = '',
    String status = 'proposed',
    required String idempotencyKey,
  }) async => _post(
    '/api/v2/decisions',
    {
      'projectId': projectId,
      'title': title,
      'context': context,
      'outcome': outcome,
      'rationale': rationale,
      'status': status,
    },
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceUpdateDecision({
    required String decisionId,
    String? projectId,
    String? title,
    String? context,
    String? outcome,
    String? rationale,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => _patch(
    '/api/v2/decisions/$decisionId',
    {
      'projectId': ?projectId,
      'title': ?title,
      'context': ?context,
      'outcome': ?outcome,
      'rationale': ?rationale,
      'status': ?status,
      'expectedVersion': expectedVersion,
    },
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceProjectHistory(String projectId) =>
      _get('/api/v2/projects/$projectId/history');

  Future<Map<String, dynamic>> workspaceTaskHistory(String taskId) =>
      _get('/api/v2/tasks/$taskId/history');

  Future<Map<String, dynamic>> workspaceDecisionHistory(String decisionId) =>
      _get('/api/v2/decisions/$decisionId/history');

  Future<Map<String, dynamic>> workspaceStateMutation({
    required String path,
    required int expectedVersion,
    required String idempotencyKey,
  }) async => _post(
    path,
    {'expectedVersion': expectedVersion},
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceImportManualSource({
    required String name,
    required String content,
    required String idempotencyKey,
    String kind = 'manual',
    String uri = '',
    String mimeType = 'text/plain',
  }) async => _post(
    '/api/v2/knowledge/sources',
    {
      'name': name,
      'content': content,
      'kind': kind,
      'uri': uri,
      'mimeType': mimeType,
    },
    extraHeaders: {'Idempotency-Key': idempotencyKey},
  );

  Future<Map<String, dynamic>> workspaceKnowledgeSourceDetail(
    String sourceId,
  ) => _get('/api/v2/knowledge/sources/$sourceId/detail');

  Future<Map<String, dynamic>> workspaceSearchKnowledge(
    String query, {
    int limit = 20,
  }) async {
    final encodedQuery = Uri.encodeQueryComponent(query.trim());
    return _get(
      '/api/v2/knowledge/search?q=$encodedQuery&limit=$limit',
      timeout: const Duration(seconds: 15),
    );
  }

  Future<Map<String, dynamic>> workspaceSyncDrive({
    String cursor = '',
    int pageSize = 100,
  }) async => _post('/api/v2/connectors/drive/sync', {
    'cursor': cursor,
    'pageSize': pageSize,
  }, timeout: const Duration(seconds: 90));

  Future<Map<String, dynamic>> workspaceSyncGitHub({
    required String repository,
    String cursor = '',
    int pageSize = 100,
  }) async => _post('/api/v2/connectors/github/sync', {
    'repository': repository,
    'cursor': cursor,
    'pageSize': pageSize,
  }, timeout: const Duration(seconds: 90));

  Future<Map<String, dynamic>> workspaceStartConversation(
    String message, {
    Map<String, dynamic>? context,
  }) async => _post('/api/v2/assistant/conversations', {
    'message': message,
    ...?context,
  });

  Future<Map<String, dynamic>> workspaceSendMessage(
    String conversationId,
    String message,
  ) async => _post('/api/v2/assistant/conversations/$conversationId/messages', {
    'message': message,
  });

  Future<Map<String, dynamic>> workspaceListConversations({
    int limit = 20,
  }) async => _get(
    '/api/v2/assistant/conversations?limit=$limit',
    timeout: const Duration(seconds: 15),
  );

  Future<Map<String, dynamic>> workspaceGetConversation(
    String conversationId,
  ) async => _get(
    '/api/v2/assistant/conversations/${Uri.encodeComponent(conversationId)}',
    timeout: const Duration(seconds: 15),
  );

  Future<Map<String, dynamic>> workspaceCreateActionChallenge(
    Map<String, dynamic> body,
  ) async => _post('/api/v2/actions/challenges', body);

  Future<Map<String, dynamic>> workspaceConfirmAction(
    Map<String, dynamic> body,
  ) async => _post('/api/v2/actions/confirm', body);

  Future<Map<String, dynamic>> workspaceRecordLearningObservation({
    required String sourceType,
    String sourceId = '',
    required String skill,
    required String prompt,
    required String response,
    String feedback = '',
  }) async => _post('/api/v2/learning/observations', {
    'sourceType': sourceType,
    'sourceId': sourceId,
    'skill': skill,
    'prompt': prompt,
    'response': response,
    'feedback': feedback,
  });

  Future<Map<String, dynamic>> workspaceListLearningObservations({
    int limit = 20,
  }) => _get('/api/v2/learning/observations?limit=$limit');

  Future<Map<String, dynamic>> _get(
    String path, {
    Duration timeout = const Duration(seconds: 5),
  }) async {
    final response = await _client
        .get(Uri.parse('$baseUrl$path'), headers: _headers())
        .timeout(timeout);
    return _decode(response);
  }

  Future<Map<String, dynamic>> _post(
    String path,
    Map<String, dynamic> body, {
    Duration timeout = const Duration(seconds: 20),
    Map<String, String>? extraHeaders,
  }) async {
    final response = await _client
        .post(
          Uri.parse('$baseUrl$path'),
          headers: _headers({
            'content-type': 'application/json',
            ...?extraHeaders,
          }),
          body: jsonEncode(body),
        )
        .timeout(timeout);
    return _decode(response);
  }

  Future<Map<String, dynamic>> _patch(
    String path,
    Map<String, dynamic> body, {
    Duration timeout = const Duration(seconds: 20),
    Map<String, String>? extraHeaders,
  }) async {
    final response = await _client
        .patch(
          Uri.parse('$baseUrl$path'),
          headers: _headers({
            'content-type': 'application/json',
            ...?extraHeaders,
          }),
          body: jsonEncode(body),
        )
        .timeout(timeout);
    return _decode(response);
  }

  Future<Map<String, dynamic>> _put(
    String path,
    Map<String, dynamic> body,
  ) async {
    final response = await _client
        .put(
          Uri.parse('$baseUrl$path'),
          headers: _headers({'content-type': 'application/json'}),
          body: jsonEncode(body),
        )
        .timeout(const Duration(seconds: 10));
    return _decode(response);
  }

  Future<Map<String, dynamic>> _delete(String path) async {
    final response = await _client
        .delete(Uri.parse('$baseUrl$path'), headers: _headers())
        .timeout(const Duration(seconds: 10));
    return _decode(response);
  }

  Future<Map<String, dynamic>> _multipart(
    String path,
    List<int> bytes,
    String mimeType,
    Map<String, String> fields,
  ) async {
    final request = http.MultipartRequest('POST', Uri.parse('$baseUrl$path'))
      ..fields.addAll(fields)
      ..files.add(
        http.MultipartFile.fromBytes(
          'audio',
          bytes,
          filename: mimeType.contains('wav')
              ? 'recording.wav'
              : 'recording.webm',
        ),
      );
    request.headers.addAll(_headers());
    final response = await _client.send(request).then(http.Response.fromStream);
    return _decode(response);
  }

  Map<String, dynamic> _decode(http.Response response) {
    final decoded = response.body.trim().isEmpty
        ? <String, dynamic>{}
        : jsonDecode(response.body);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      final envelope = _map(decoded);
      final rawError = envelope['error'];
      final message = rawError is Map
          ? rawError['message']?.toString() ?? 'Request failed'
          : rawError?.toString() ?? 'Request failed';
      final code = rawError is Map ? rawError['code']?.toString() : null;
      if (response.statusCode == 401) {
        onUnauthorized?.call();
        throw AuthRequiredException(message);
      }
      throw ApiException(response.statusCode, message, code: code);
    }
    return _map(decoded);
  }

  Map<String, String> _headers([Map<String, String>? initial]) {
    return <String, String>{...?initial};
  }
}

List<Map<String, dynamic>> _list(dynamic value) => value is List
    ? value
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList()
    : <Map<String, dynamic>>[];
Map<String, dynamic> _map(dynamic value) =>
    value is Map<String, dynamic> ? value : <String, dynamic>{};
