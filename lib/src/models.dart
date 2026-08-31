class AuthUser {
  const AuthUser({
    required this.id,
    required this.displayName,
    required this.cefr,
  });

  final String id;
  final String displayName;
  final String cefr;

  factory AuthUser.fromJson(Map<String, dynamic> json) => AuthUser(
    id: _string(json['id'], 'user-1'),
    displayName: _string(json['displayName'], 'Developer'),
    cefr: _string(json['cefr'], 'A1'),
  );
}

class Mission {
  const Mission({
    required this.id,
    required this.title,
    required this.mode,
    required this.skillLabel,
    required this.level,
    required this.context,
    required this.prompt,
    required this.targetVocabulary,
    required this.estimatedMinutes,
  });

  final String id;
  final String title;
  final String mode;
  final String skillLabel;
  final String level;
  final String context;
  final String prompt;
  final List<String> targetVocabulary;
  final int estimatedMinutes;

  factory Mission.fromJson(Map<String, dynamic> json) => Mission(
    id: _string(json['id'], 'mission-today'),
    title: _string(json['title'], 'Today\'s technical mission'),
    mode: _string(json['mode'], 'writing'),
    skillLabel: _string(json['skillLabel'], 'Technical Writing'),
    level: _string(json['level'], 'B1'),
    context: _string(json['context'], ''),
    prompt: _string(json['prompt'], ''),
    targetVocabulary: _strings(json['targetVocabulary']),
    estimatedMinutes: _int(json['estimatedMinutes'], 10),
  );
}

class SkillState {
  const SkillState({
    required this.label,
    required this.score,
    required this.trend,
    required this.isWeakest,
  });

  final String label;
  final double score;
  final double trend;
  final bool isWeakest;

  factory SkillState.fromJson(Map<String, dynamic> json) => SkillState(
    label: _string(json['label'], _string(json['skill'], 'Skill')),
    score: _double(json['score']),
    trend: _double(json['trend']),
    isWeakest: json['isWeakest'] == true,
  );
}

class LearningState {
  const LearningState({
    required this.cefr,
    required this.overallScore,
    required this.weakestSkill,
    required this.skills,
  });

  final String cefr;
  final double overallScore;
  final String weakestSkill;
  final List<SkillState> skills;

  factory LearningState.fromJson(Map<String, dynamic> json) => LearningState(
    cefr: _string(json['cefr'], 'B1'),
    overallScore: _double(json['overallScore']),
    weakestSkill: _string(json['weakestSkill'], 'technical_writing'),
    skills: _maps(json['skills']).map(SkillState.fromJson).toList(),
  );
}

class ProviderStatus {
  const ProviderStatus({required this.name, required this.configured});

  final String name;
  final bool configured;

  factory ProviderStatus.fromJson(Map<String, dynamic> json) => ProviderStatus(
    name: _string(json['name'], 'deterministic-fallback'),
    configured: json['configured'] == true,
  );
}

class SettingsData {
  const SettingsData({
    required this.aiProvider,
    required this.fastModel,
    required this.smartModel,
    required this.deepSeekConfigured,
    required this.deepSeekStatus,
    required this.speechConfigured,
    required this.pronunciationOn,
    required this.monthlyBudgetVnd,
  });

  final String aiProvider;
  final String fastModel;
  final String smartModel;
  final bool deepSeekConfigured;
  final String deepSeekStatus;
  final bool speechConfigured;
  final bool pronunciationOn;
  final int monthlyBudgetVnd;

  factory SettingsData.fromJson(Map<String, dynamic> json) => SettingsData(
    aiProvider: _string(json['aiProvider'], 'deepseek'),
    fastModel: _string(json['fastModel'], 'deepseek-v4-flash'),
    smartModel: _string(json['smartModel'], 'deepseek-v4-pro'),
    deepSeekConfigured: json['deepSeekConfigured'] == true,
    deepSeekStatus: _string(json['deepSeekStatus'], 'not_configured'),
    speechConfigured: json['speechConfigured'] == true,
    pronunciationOn: json['pronunciationOn'] == true,
    monthlyBudgetVnd: _int(json['monthlyBudgetVnd'], 150000),
  );

  SettingsData copyWith({
    String? fastModel,
    String? smartModel,
    bool? pronunciationOn,
  }) => SettingsData(
    aiProvider: aiProvider,
    fastModel: fastModel ?? this.fastModel,
    smartModel: smartModel ?? this.smartModel,
    deepSeekConfigured: deepSeekConfigured,
    deepSeekStatus: deepSeekStatus,
    speechConfigured: speechConfigured,
    pronunciationOn: pronunciationOn ?? this.pronunciationOn,
    monthlyBudgetVnd: monthlyBudgetVnd,
  );
}

class ProgressPoint {
  const ProgressPoint({required this.date, required this.score});

  final String date;
  final double score;

  factory ProgressPoint.fromJson(Map<String, dynamic> json) => ProgressPoint(
    date: _string(json['date'], ''),
    score: _double(json['score']),
  );
}

class HomeData {
  const HomeData({
    required this.name,
    required this.state,
    required this.mission,
    required this.reviewDueCount,
    required this.progress,
    required this.provider,
  });

  final String name;
  final LearningState state;
  final Mission mission;
  final int reviewDueCount;
  final List<ProgressPoint> progress;
  final ProviderStatus provider;

  factory HomeData.fromJson(Map<String, dynamic> json) => HomeData(
    name: _string(_map(json['user'])['displayName'], 'Developer'),
    state: LearningState.fromJson(_map(json['learningState'])),
    mission: Mission.fromJson(_map(json['todayMission'])),
    reviewDueCount: _int(json['reviewDueCount'], 0),
    progress: _maps(
      json['recentProgress'],
    ).map(ProgressPoint.fromJson).toList(),
    provider: ProviderStatus.fromJson(_map(json['provider'])),
  );
}

class PracticeMode {
  const PracticeMode({
    required this.id,
    required this.title,
    required this.description,
    required this.minutes,
    required this.available,
  });

  final String id;
  final String title;
  final String description;
  final int minutes;
  final bool available;

  factory PracticeMode.fromJson(Map<String, dynamic> json) => PracticeMode(
    id: _string(json['id'], ''),
    title: _string(json['title'], ''),
    description: _string(json['description'], ''),
    minutes: _int(json['minutes'], 10),
    available: json['available'] != false,
  );
}

class ReviewItem {
  const ReviewItem({
    required this.id,
    required this.kind,
    required this.prompt,
    required this.answer,
    required this.context,
    required this.level,
  });

  final String id;
  final String kind;
  final String prompt;
  final String answer;
  final String context;
  final String level;

  factory ReviewItem.fromJson(Map<String, dynamic> json) => ReviewItem(
    id: _string(json['id'], ''),
    kind: _string(json['kind'], 'review'),
    prompt: _string(json['prompt'], ''),
    answer: _string(json['answer'], ''),
    context: _string(json['context'], ''),
    level: _string(json['level'], 'B1'),
  );
}

class ProgressData {
  const ProgressData({
    required this.state,
    required this.repeatedMistakes,
    required this.technicalScore,
    required this.trend,
  });

  final LearningState state;
  final int repeatedMistakes;
  final double technicalScore;
  final List<ProgressPoint> trend;

  factory ProgressData.fromJson(Map<String, dynamic> json) => ProgressData(
    state: LearningState.fromJson(_map(json['learningState'])),
    repeatedMistakes: _int(json['repeatedMistakes'], 0),
    technicalScore: _double(json['technicalScore']),
    trend: _maps(json['speakingTrend']).map(ProgressPoint.fromJson).toList(),
  );
}

class Correction {
  const Correction({
    required this.original,
    required this.corrected,
    required this.why,
  });

  final String original;
  final String corrected;
  final String why;

  factory Correction.fromJson(Map<String, dynamic> json) => Correction(
    original: _string(json['original'], ''),
    corrected: _string(json['corrected'], ''),
    why: _string(json['why'], ''),
  );
}

class EvaluationResult {
  const EvaluationResult({
    required this.score,
    required this.summary,
    required this.good,
    required this.mainIssue,
    required this.nextAction,
    required this.corrections,
  });

  final double score;
  final String summary;
  final List<String> good;
  final String mainIssue;
  final String nextAction;
  final List<Correction> corrections;

  factory EvaluationResult.fromJson(Map<String, dynamic> json) =>
      EvaluationResult(
        score: _double(json['score']),
        summary: _string(json['summary'], ''),
        good: _strings(json['whatWasGood']),
        mainIssue: _string(json['mainIssue'], ''),
        nextAction: _string(json['nextAction'], ''),
        corrections: _maps(
          json['corrections'],
        ).map(Correction.fromJson).toList(),
      );
}

class SubmissionResult {
  const SubmissionResult({
    required this.evaluation,
    required this.mistakeCount,
  });

  final EvaluationResult evaluation;
  final int mistakeCount;

  factory SubmissionResult.fromJson(Map<String, dynamic> json) =>
      SubmissionResult(
        evaluation: EvaluationResult.fromJson(_map(json['evaluation'])),
        mistakeCount: _maps(json['mistakes']).length,
      );
}

class WorkImportResult {
  const WorkImportResult({
    required this.domain,
    required this.terms,
    required this.mission,
    this.sourceUrl = '',
  });

  final String domain;
  final List<String> terms;
  final Mission mission;
  final String sourceUrl;

  factory WorkImportResult.fromJson(Map<String, dynamic> json) =>
      WorkImportResult(
        domain: _string(
          _map(json['context'])['domain'],
          'software-engineering',
        ),
        terms: _strings(json['suggestedTerms']),
        mission: Mission.fromJson(_map(json['suggestedMission'])),
        sourceUrl: _string(_map(json['context'])['sourceUrl'], ''),
      );
}

class DiagnosticQuestion {
  const DiagnosticQuestion({
    required this.id,
    required this.skill,
    required this.mode,
    required this.prompt,
    required this.options,
  });

  final String id;
  final String skill;
  final String mode;
  final String prompt;
  final List<String> options;

  factory DiagnosticQuestion.fromJson(Map<String, dynamic> json) =>
      DiagnosticQuestion(
        id: _string(json['id'], ''),
        skill: _string(json['skill'], ''),
        mode: _string(json['mode'], 'writing'),
        prompt: _string(json['prompt'], ''),
        options: _strings(json['options']),
      );
}

class DiagnosticResult {
  const DiagnosticResult({
    required this.cefr,
    required this.overallScore,
    required this.strengths,
    required this.priorities,
    required this.recommendedPlan,
  });

  final String cefr;
  final double overallScore;
  final List<String> strengths;
  final List<String> priorities;
  final List<String> recommendedPlan;

  factory DiagnosticResult.fromJson(Map<String, dynamic> json) =>
      DiagnosticResult(
        cefr: _string(json['cefr'], 'A1'),
        overallScore: _double(json['overallScore']),
        strengths: _strings(json['strengths']),
        priorities: _strings(json['priorities']),
        recommendedPlan: _strings(json['recommendedPlan']),
      );
}

class RoleplayScenario {
  const RoleplayScenario({
    required this.id,
    required this.title,
    required this.partnerRole,
    required this.level,
    required this.context,
    required this.goal,
    this.type = 'roleplay',
  });

  final String id;
  final String title;
  final String partnerRole;
  final String level;
  final String context;
  final String goal;
  final String type;

  factory RoleplayScenario.fromJson(Map<String, dynamic> json) =>
      RoleplayScenario(
        id: _string(json['id'], ''),
        title: _string(json['title'], ''),
        partnerRole: _string(json['partnerRole'], 'Teammate'),
        level: _string(json['level'], 'B1'),
        context: _string(json['context'], ''),
        goal: _string(json['goal'], ''),
        type: _string(json['type'], 'roleplay'),
      );
}

class Message {
  const Message({required this.id, required this.role, required this.content});

  final String id;
  final String role;
  final String content;

  factory Message.fromJson(Map<String, dynamic> json) => Message(
    id: _string(json['id'], ''),
    role: _string(json['role'], 'assistant'),
    content: _string(json['content'], ''),
  );
}

class Conversation {
  const Conversation({
    required this.id,
    required this.roleplayType,
    required this.context,
    required this.messages,
  });

  final String id;
  final String roleplayType;
  final String context;
  final List<Message> messages;

  factory Conversation.fromJson(Map<String, dynamic> json) => Conversation(
    id: _string(json['id'], ''),
    roleplayType: _string(json['roleplayType'], ''),
    context: _string(json['context'], ''),
    messages: _maps(json['messages']).map(Message.fromJson).toList(),
  );
}

class RoleplayTurnResult {
  const RoleplayTurnResult({
    required this.conversation,
    required this.feedback,
  });

  final Conversation conversation;
  final EvaluationResult feedback;

  factory RoleplayTurnResult.fromJson(Map<String, dynamic> json) =>
      RoleplayTurnResult(
        conversation: Conversation.fromJson(_map(json['conversation'])),
        feedback: EvaluationResult.fromJson(_map(json['feedback'])),
      );
}

class CopilotResult {
  const CopilotResult({
    required this.simple,
    required this.natural,
    required this.professional,
    required this.explanation,
  });

  final String simple;
  final String natural;
  final String professional;
  final String explanation;

  factory CopilotResult.fromJson(Map<String, dynamic> json) => CopilotResult(
    simple: _string(json['simple'], ''),
    natural: _string(json['natural'], ''),
    professional: _string(json['professional'], ''),
    explanation: _string(json['explanation'], ''),
  );
}

class VocabularyItem {
  const VocabularyItem({
    required this.id,
    required this.term,
    required this.domain,
    required this.level,
    required this.definition,
    required this.example,
    required this.mastery,
    this.relatedTerms = const [],
  });

  final String id;
  final String term;
  final String domain;
  final String level;
  final String definition;
  final String example;
  final double mastery;
  final List<String> relatedTerms;

  factory VocabularyItem.fromJson(Map<String, dynamic> json) => VocabularyItem(
    id: _string(json['id'], ''),
    term: _string(json['term'], ''),
    domain: _string(json['domain'], 'software-engineering'),
    level: _string(json['level'], 'B1'),
    definition: _string(json['definition'], ''),
    example: _string(json['technicalExample'], ''),
    mastery: _double(json['mastery']),
    relatedTerms: _strings(json['relatedTerms']),
  );
}

class VocabularyGraph {
  const VocabularyGraph({required this.nodes, required this.edges});

  final List<VocabularyGraphNode> nodes;
  final List<VocabularyGraphEdge> edges;

  factory VocabularyGraph.fromJson(Map<String, dynamic> json) =>
      VocabularyGraph(
        nodes: _maps(json['nodes']).map(VocabularyGraphNode.fromJson).toList(),
        edges: _maps(json['edges']).map(VocabularyGraphEdge.fromJson).toList(),
      );
}

class VocabularyGraphNode {
  const VocabularyGraphNode({
    required this.id,
    required this.label,
    required this.mastery,
  });

  final String id;
  final String label;
  final double mastery;

  factory VocabularyGraphNode.fromJson(Map<String, dynamic> json) =>
      VocabularyGraphNode(
        id: _string(json['id'], ''),
        label: _string(json['label'], ''),
        mastery: _double(json['mastery']),
      );
}

class VocabularyGraphEdge {
  const VocabularyGraphEdge({
    required this.from,
    required this.to,
    required this.relation,
  });

  final String from;
  final String to;
  final String relation;

  factory VocabularyGraphEdge.fromJson(Map<String, dynamic> json) =>
      VocabularyGraphEdge(
        from: _string(json['from'], ''),
        to: _string(json['to'], ''),
        relation: _string(json['relation'], 'related'),
      );
}

class WeeklySpeakingAssessment {
  const WeeklySpeakingAssessment({
    required this.week,
    required this.sessions,
    required this.evaluated,
    required this.averageScore,
    required this.averageFluency,
    required this.averageProsody,
    required this.recommendation,
  });

  final String week;
  final int sessions;
  final int evaluated;
  final double averageScore;
  final double averageFluency;
  final double averageProsody;
  final String recommendation;

  factory WeeklySpeakingAssessment.fromJson(Map<String, dynamic> json) =>
      WeeklySpeakingAssessment(
        week: _string(json['week'], ''),
        sessions: _int(json['sessions'], 0),
        evaluated: _int(json['evaluated'], 0),
        averageScore: _double(json['averageScore']),
        averageFluency: _double(json['averageFluency']),
        averageProsody: _double(json['averageProsody']),
        recommendation: _string(json['recommendation'], ''),
      );
}

class AnalyticsSummary {
  const AnalyticsSummary({
    required this.windowDays,
    required this.minutesLearned,
    required this.missionsCompleted,
    required this.missionsCreated,
    required this.repeatedMistakes,
    required this.technicalCommunicationScore,
    required this.vocabularyMastery,
    required this.weeklySpeaking,
  });

  final int windowDays;
  final int minutesLearned;
  final int missionsCompleted;
  final int missionsCreated;
  final int repeatedMistakes;
  final double technicalCommunicationScore;
  final double vocabularyMastery;
  final WeeklySpeakingAssessment weeklySpeaking;

  factory AnalyticsSummary.fromJson(Map<String, dynamic> json) =>
      AnalyticsSummary(
        windowDays: _int(json['windowDays'], 7),
        minutesLearned: _int(json['minutesLearned'], 0),
        missionsCompleted: _int(json['missionsCompleted'], 0),
        missionsCreated: _int(json['missionsCreated'], 0),
        repeatedMistakes: _int(json['repeatedMistakes'], 0),
        technicalCommunicationScore: _double(
          json['technicalCommunicationScore'],
        ),
        vocabularyMastery: _double(json['vocabularyMastery']),
        weeklySpeaking: WeeklySpeakingAssessment.fromJson(
          _map(json['weeklySpeaking']),
        ),
      );
}

class ProviderCheck {
  const ProviderCheck({
    required this.provider,
    required this.configured,
    required this.status,
  });

  final String provider;
  final bool configured;
  final String status;

  factory ProviderCheck.fromJson(Map<String, dynamic> json) => ProviderCheck(
    provider: _string(json['provider'], 'Provider'),
    configured: json['configured'] == true,
    status: _string(json['status'], 'not_configured'),
  );
}

class UsageSummary {
  const UsageSummary({
    required this.month,
    required this.estimatedCost,
    required this.budgetVnd,
    required this.budgetUsedPercent,
    required this.unavailableRecords,
  });

  final String month;
  final double estimatedCost;
  final int budgetVnd;
  final double budgetUsedPercent;
  final int unavailableRecords;

  factory UsageSummary.fromJson(Map<String, dynamic> json) => UsageSummary(
    month: _string(json['month'], ''),
    estimatedCost: _double(json['estimatedCost']),
    budgetVnd: _int(json['budgetVnd'], 150000),
    budgetUsedPercent: _double(json['budgetUsedPercent']),
    unavailableRecords: _int(json['unavailableRecords'], 0),
  );
}

class SpeakingSession {
  const SpeakingSession({
    required this.id,
    required this.status,
    required this.transcript,
    this.pronunciation,
  });

  final String id;
  final String status;
  final String transcript;
  final PronunciationScore? pronunciation;

  factory SpeakingSession.fromJson(Map<String, dynamic> json) =>
      SpeakingSession(
        id: _string(json['id'], ''),
        status: _string(json['status'], 'transcribed'),
        transcript: _string(json['transcript'], ''),
        pronunciation: json['pronunciation'] is Map
            ? PronunciationScore.fromJson(_map(json['pronunciation']))
            : null,
      );
}

class PronunciationScore {
  const PronunciationScore({
    required this.score,
    required this.accuracy,
    required this.fluency,
    required this.completeness,
    required this.prosody,
    this.words = const {},
  });

  final double score;
  final double accuracy;
  final double fluency;
  final double completeness;
  final double prosody;
  final Map<String, double> words;

  factory PronunciationScore.fromJson(Map<String, dynamic> json) =>
      PronunciationScore(
        score: _double(json['score']),
        accuracy: _double(json['accuracy']),
        fluency: _double(json['fluency']),
        completeness: _double(json['completeness']),
        prosody: _double(json['prosody']),
        words: _doubleMap(json['words']),
      );
}

Map<String, dynamic> _map(dynamic value) =>
    value is Map<String, dynamic> ? value : <String, dynamic>{};
List<Map<String, dynamic>> _maps(dynamic value) => value is List
    ? value
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList()
    : <Map<String, dynamic>>[];
List<String> _strings(dynamic value) =>
    value is List ? value.map((item) => item.toString()).toList() : <String>[];
String _string(dynamic value, String fallback) =>
    value is String && value.trim().isNotEmpty ? value : fallback;
int _int(dynamic value, int fallback) =>
    value is num ? value.toInt() : fallback;
double _double(dynamic value) => value is num ? value.toDouble() : 0;
Map<String, double> _doubleMap(dynamic value) => value is Map
    ? value.map((key, item) => MapEntry(key.toString(), _double(item)))
    : <String, double>{};
