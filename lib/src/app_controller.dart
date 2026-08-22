import 'package:flutter/foundation.dart';

import 'api.dart';
import 'demo_data.dart';
import 'models.dart';

const bool _allowDemoFallback =
    String.fromEnvironment('DEVENGLISH_ENV', defaultValue: 'development') !=
    'production';
const VocabularyGraph _emptyVocabularyGraph = VocabularyGraph(
  nodes: [],
  edges: [],
);

class AppController extends ChangeNotifier {
  AppController({DevEnglishApi? api}) : _api = api ?? DevEnglishApi();

  final DevEnglishApi _api;
  HomeData _home = DemoData.home;
  List<PracticeMode> _practice = DemoData.practice;
  List<ReviewItem> _review = DemoData.review;
  ProgressData _progress = DemoData.progress;
  bool _loading = true;
  bool _submitting = false;
  bool _importing = false;
  bool _usingDemo = true;
  String? _error;
  SubmissionResult? _lastSubmission;
  WorkImportResult? _lastWorkImport;
  List<DiagnosticQuestion> _diagnosticQuestions = DemoData.diagnosticQuestions;
  DiagnosticResult? _diagnosticResult;
  List<RoleplayScenario> _scenarios = DemoData.roleplayScenarios;
  Conversation? _conversation;
  RoleplayTurnResult? _lastRoleplayTurn;
  CopilotResult? _copilotResult;
  List<VocabularyItem> _vocabulary = const [];
  VocabularyGraph _vocabularyGraph = DemoData.vocabularyGraph;
  UsageSummary? _usageSummary;
  SettingsData _settings = DemoData.settings;
  AnalyticsSummary _analytics = DemoData.analytics;
  WeeklySpeakingAssessment _weeklySpeaking = DemoData.weeklySpeaking;
  List<ProviderCheck> _providerChecks = DemoData.providerChecks;
  SpeakingSession? _speakingSession;
  bool _working = false;

  HomeData get home => _home;
  List<PracticeMode> get practice => _practice;
  List<ReviewItem> get review => _review;
  ProgressData get progress => _progress;
  bool get loading => _loading;
  bool get submitting => _submitting;
  bool get importing => _importing;
  bool get usingDemo => _usingDemo;
  bool get demoFallbackEnabled => _allowDemoFallback;
  String? get error => _error;
  SubmissionResult? get lastSubmission => _lastSubmission;
  WorkImportResult? get lastWorkImport => _lastWorkImport;
  List<DiagnosticQuestion> get diagnosticQuestions => _diagnosticQuestions;
  DiagnosticResult? get diagnosticResult => _diagnosticResult;
  List<RoleplayScenario> get scenarios => _scenarios;
  Conversation? get conversation => _conversation;
  RoleplayTurnResult? get lastRoleplayTurn => _lastRoleplayTurn;
  CopilotResult? get copilotResult => _copilotResult;
  List<VocabularyItem> get vocabulary => _vocabulary;
  VocabularyGraph get vocabularyGraph => _vocabularyGraph;
  UsageSummary? get usageSummary => _usageSummary;
  SettingsData get settings => _settings;
  AnalyticsSummary get analytics => _analytics;
  WeeklySpeakingAssessment get weeklySpeaking => _weeklySpeaking;
  List<ProviderCheck> get providerChecks => _providerChecks;
  SpeakingSession? get speakingSession => _speakingSession;
  bool get working => _working;

  Future<void> load() async {
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final results = await Future.wait<dynamic>([
        _api.home(),
        _api.practice(),
        _api.reviewDue(),
        _api.progress(),
      ]);
      _home = results[0] as HomeData;
      _practice = results[1] as List<PracticeMode>;
      _review = results[2] as List<ReviewItem>;
      _progress = results[3] as ProgressData;
      _usingDemo = false;
      try {
        _diagnosticResult = await _api.diagnosticResult();
      } catch (_) {
        _diagnosticResult = null;
      }
      try {
        final results = await Future.wait<dynamic>([
          _api.analytics(),
          _api.weeklySpeaking(),
        ]);
        _analytics = results[0] as AnalyticsSummary;
        _weeklySpeaking = results[1] as WeeklySpeakingAssessment;
      } catch (_) {
        if (_allowDemoFallback) {
          _analytics = DemoData.analytics;
          _weeklySpeaking = DemoData.weeklySpeaking;
        } else {
          _usingDemo = true;
          _error = 'Không thể tải analytics production từ backend.';
        }
      }
    } catch (_) {
      _usingDemo = true;
      _error = _allowDemoFallback
          ? 'Backend chưa chạy, đang hiển thị dữ liệu demo cục bộ.'
          : 'Không thể kết nối backend production. Vui lòng thử lại.';
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  Future<void> submitWriting(String missionId, String answer) async {
    _submitting = true;
    _error = null;
    _lastSubmission = null;
    notifyListeners();
    try {
      _lastSubmission = await _api.submitWriting(missionId, answer);
    } catch (_) {
      if (_allowDemoFallback) {
        _lastSubmission = _localEvaluation(answer);
        _error = 'Đang dùng feedback local vì backend chưa sẵn sàng.';
      } else {
        _error = 'Không thể chấm bài từ backend production.';
      }
    } finally {
      _submitting = false;
      notifyListeners();
    }
  }

  Future<void> continueToNextMission() async {
    await load();
    _lastSubmission = null;
    notifyListeners();
  }

  Future<void> refreshAfterMissionExit() async {
    _lastSubmission = null;
    notifyListeners();
    await load();
  }

  Future<void> importWork({
    required String sourceType,
    required String title,
    required String content,
  }) async {
    _importing = true;
    _error = null;
    _lastWorkImport = null;
    notifyListeners();
    try {
      _lastWorkImport = await _api.importWork(
        sourceType: sourceType,
        title: title,
        content: content,
      );
    } catch (_) {
      _error =
          'Không thể phân tích work context lúc này. Hãy thử lại khi backend đang chạy.';
    } finally {
      _importing = false;
      notifyListeners();
    }
  }

  Future<void> importGitHub(String url) async {
    _importing = true;
    _error = null;
    _lastWorkImport = null;
    notifyListeners();
    try {
      _lastWorkImport = await _api.importGitHub(url);
    } catch (_) {
      _error =
          'Không thể import GitHub lúc này. Hãy kiểm tra URL công khai và cấu hình backend.';
    } finally {
      _importing = false;
      notifyListeners();
    }
  }

  Future<void> loadDiagnostic() async {
    try {
      _diagnosticQuestions = await _api.diagnosticQuestions();
      notifyListeners();
    } catch (_) {
      if (!_allowDemoFallback) _diagnosticQuestions = const [];
      _error = 'Không thể tải bài diagnostic lúc này.';
      notifyListeners();
    }
  }

  Future<void> submitDiagnostic(Map<String, String> answers) async {
    _working = true;
    _error = null;
    notifyListeners();
    try {
      _diagnosticResult = await _api.submitDiagnostic(
        answers.entries
            .map((entry) => {'questionId': entry.key, 'answer': entry.value})
            .toList(),
      );
      await load();
    } catch (_) {
      if (_allowDemoFallback) {
        _diagnosticResult = const DiagnosticResult(
          cefr: 'B1',
          overallScore: 58,
          strengths: ['You can describe concrete technical situations.'],
          priorities: ['Technical writing', 'Speaking structure'],
          recommendedPlan: [
            'Write one 10-minute bug report each day.',
            'Review two mistakes after each mission.',
            'Explain one technical decision before the weekly checkpoint.',
          ],
        );
      }
      _error = 'Không thể lưu kết quả diagnostic lúc này.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> loadRoleplay() async {
    try {
      _scenarios = await _api.roleplayScenarios();
      notifyListeners();
    } catch (_) {
      if (!_allowDemoFallback) _scenarios = const [];
      _error = 'Không thể tải roleplay scenarios lúc này.';
      notifyListeners();
    }
  }

  Future<void> startRoleplay(String scenarioId) async {
    _working = true;
    _error = null;
    notifyListeners();
    try {
      _conversation = await _api.startRoleplay(scenarioId);
      _lastRoleplayTurn = null;
    } catch (_) {
      if (!_allowDemoFallback) {
        _error = 'Không thể bắt đầu roleplay production lúc này.';
        return;
      }
      final scenario = _scenarios.firstWhere(
        (item) => item.id == scenarioId,
        orElse: () => DemoData.roleplayScenarios.first,
      );
      _conversation = Conversation(
        id: 'demo-conversation',
        roleplayType: scenario.id,
        context: scenario.context,
        messages: [
          Message(
            id: 'demo-opening',
            role: 'assistant',
            content:
                'Walk me through what happened, the user impact and your next step.',
          ),
        ],
      );
      _error = 'Không thể bắt đầu roleplay lúc này.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> sendRoleplayTurn(String answer) async {
    final conversation = _conversation;
    if (conversation == null || answer.trim().isEmpty) return;
    _working = true;
    _error = null;
    notifyListeners();
    try {
      _lastRoleplayTurn = await _api.roleplayTurn(conversation.id, answer);
      _conversation = _lastRoleplayTurn!.conversation;
    } catch (_) {
      if (!_allowDemoFallback) {
        _error = 'Không thể gửi lượt roleplay production lúc này.';
        return;
      }
      final reply = answer.toLowerCase().contains('impact')
          ? 'Good. How will you validate the fix and communicate the result?'
          : 'What evidence supports that explanation, and what is the user impact?';
      final evaluation = EvaluationResult(
        score: answer.trim().split(RegExp(r'\s+')).length >= 12 ? 72 : 58,
        summary: 'The explanation is understandable and work-focused.',
        good: const ['You responded to the technical context.'],
        mainIssue: 'Connect evidence, impact and next step more explicitly.',
        nextAction: 'Answer once more with evidence, impact and next step.',
        corrections: const [],
      );
      _conversation = Conversation(
        id: conversation.id,
        roleplayType: conversation.roleplayType,
        context: conversation.context,
        messages: [
          ...conversation.messages,
          Message(
            id: 'demo-user-${conversation.messages.length}',
            role: 'user',
            content: answer,
          ),
          Message(
            id: 'demo-reply-${conversation.messages.length}',
            role: 'assistant',
            content: reply,
          ),
        ],
      );
      _lastRoleplayTurn = RoleplayTurnResult(
        conversation: _conversation!,
        feedback: evaluation,
      );
      _error = 'Không thể gửi lượt trả lời lúc này.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> runCopilot(String vietnamese, {String context = ''}) async {
    _working = true;
    _copilotResult = null;
    _error = null;
    notifyListeners();
    try {
      _copilotResult = await _api.copilot(
        vietnamese: vietnamese,
        context: context,
      );
    } catch (_) {
      if (!_allowDemoFallback) {
        _error = 'Không thể tạo English Copilot từ backend production.';
        return;
      }
      final action = vietnamese.toLowerCase().contains('kiểm tra')
          ? 'check the API again'
          : 'review this request';
      _copilotResult = CopilotResult(
        simple: 'Please $action.',
        natural: 'Could you please $action?',
        professional: 'Could you please $action and share the result?',
        explanation:
            'The demo fallback keeps the intent and offers three levels of developer-facing wording.',
      );
      _error = 'Không thể tạo English Copilot lúc này.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> loadVocabularyAndUsage() async {
    try {
      final results = await Future.wait<dynamic>([
        _api.vocabulary(),
        _api.vocabularyGraph(),
        _api.usage(),
      ]);
      _vocabulary = results[0] as List<VocabularyItem>;
      _vocabularyGraph = results[1] as VocabularyGraph;
      _usageSummary = results[2] as UsageSummary;
      notifyListeners();
    } catch (_) {
      if (_allowDemoFallback) {
        _vocabulary = DemoData.vocabulary;
        _vocabularyGraph = DemoData.vocabularyGraph;
        _usageSummary = DemoData.usage;
      } else {
        _vocabulary = const [];
        _vocabularyGraph = _emptyVocabularyGraph;
        _usageSummary = null;
      }
      _error = 'Không thể tải vocabulary hoặc usage lúc này.';
      notifyListeners();
    }
  }

  Future<void> loadSettingsData() async {
    try {
      final results = await Future.wait<dynamic>([
        _api.usage(),
        _api.testConnections(),
        _api.settings(),
      ]);
      _usageSummary = results[0] as UsageSummary;
      _providerChecks = results[1] as List<ProviderCheck>;
      _settings = results[2] as SettingsData;
    } catch (_) {
      if (_allowDemoFallback) {
        _usageSummary = DemoData.usage;
        _providerChecks = DemoData.providerChecks;
        _settings = DemoData.settings;
      } else {
        _usageSummary = null;
        _providerChecks = const [];
        _error = 'Không thể tải cấu hình provider production lúc này.';
      }
    }
    notifyListeners();
  }

  Future<void> testConnections() async {
    try {
      _providerChecks = await _api.testConnections();
    } catch (_) {
      if (!_allowDemoFallback) _providerChecks = const [];
      _error = 'Không thể kiểm tra provider lúc này.';
    }
    notifyListeners();
  }

  Future<void> updatePronunciation(bool enabled) async {
    final next = _settings.copyWith(pronunciationOn: enabled);
    try {
      _settings = await _api.updateSettings(next);
    } catch (_) {
      _settings = next;
      _error = 'Không thể lưu pronunciation setting lúc này.';
    }
    notifyListeners();
  }

  Future<void> submitReview({
    required String kind,
    required String id,
    required bool success,
  }) async {
    try {
      await _api.submitReview(
        kind: kind,
        id: id,
        success: success,
        score: success ? 90 : 35,
      );
      _review = await _api.reviewDue();
      notifyListeners();
    } catch (_) {
      _error = 'Không thể cập nhật review lúc này.';
      notifyListeners();
    }
  }

  Future<void> transcribeAudio(List<int> bytes, String mimeType) async {
    _working = true;
    _error = null;
    notifyListeners();
    try {
      _speakingSession = await _api.transcribeAudio(
        bytes: bytes,
        mimeType: mimeType,
        missionId: _home.mission.id,
      );
    } catch (_) {
      _error = 'STT chưa sẵn sàng. Bạn có thể dán transcript để tiếp tục.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> assessSpeaking(
    List<int> bytes,
    String mimeType,
    String reference,
  ) async {
    final session = _speakingSession;
    if (session == null) return;
    _working = true;
    notifyListeners();
    try {
      _speakingSession = await _api.assessSpeaking(
        bytes: bytes,
        mimeType: mimeType,
        sessionId: session.id,
        reference: reference,
      );
    } catch (_) {
      _error = 'Pronunciation assessment chưa sẵn sàng.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<void> saveTranscript(String transcript) async {
    _working = true;
    _error = null;
    notifyListeners();
    try {
      _speakingSession = await _api.saveTranscript(
        transcript: transcript,
        missionId: _home.mission.id,
      );
    } catch (_) {
      _error = 'Không thể lưu transcript lúc này.';
    } finally {
      _working = false;
      notifyListeners();
    }
  }

  Future<List<int>?> synthesize(String text) async {
    try {
      return await _api.synthesize(text: text);
    } catch (_) {
      _error = 'TTS chưa sẵn sàng.';
      notifyListeners();
      return null;
    }
  }

  SubmissionResult _localEvaluation(String answer) {
    final lower = answer.toLowerCase();
    final corrections = <Correction>[];
    if (lower.contains('i has')) {
      corrections.add(
        const Correction(
          original: 'i has',
          corrected: 'I have',
          why: 'Use have with I.',
        ),
      );
    }
    final score = answer.trim().split(RegExp(r'\s+')).length >= 20
        ? 72.0
        : 58.0;
    return SubmissionResult(
      evaluation: EvaluationResult(
        score: score,
        summary: score >= 70
            ? 'The technical message is understandable.'
            : 'The answer has a useful starting point, but needs more structure.',
        good: const ['You described a concrete technical situation.'],
        mainIssue: corrections.isEmpty
            ? 'Connect the observed behavior to its impact.'
            : 'Apply the correction and retry once.',
        nextAction:
            'Retry with observed behavior, expected behavior, impact and next step.',
        corrections: corrections,
      ),
      mistakeCount: corrections.length,
    );
  }
}
