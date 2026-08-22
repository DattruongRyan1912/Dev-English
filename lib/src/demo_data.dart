import 'models.dart';

class DemoData {
  static HomeData get home => HomeData(
    name: 'Developer',
    state: const LearningState(
      cefr: 'B1',
      overallScore: 58,
      weakestSkill: 'technical_writing',
      skills: [
        SkillState(
          label: 'Documentation',
          score: 66,
          trend: 4,
          isWeakest: false,
        ),
        SkillState(
          label: 'Technical Writing',
          score: 48,
          trend: 2,
          isWeakest: true,
        ),
        SkillState(label: 'Speaking', score: 57, trend: 5, isWeakest: false),
        SkillState(label: 'Meeting', score: 61, trend: 1, isWeakest: false),
      ],
    ),
    mission: const Mission(
      id: 'mission-today',
      title: 'Write a useful bug report',
      mode: 'writing',
      skillLabel: 'Technical Writing',
      level: 'B1',
      context:
          'Your API returns HTTP 500 when a user uploads a file larger than 10 MB.',
      prompt:
          'Write a concise bug report for the backend team. Include the observed behavior, expected behavior, likely impact, and one useful next step.',
      targetVocabulary: [
        'reproduce',
        'expected behavior',
        'root cause',
        'impact',
      ],
      estimatedMinutes: 10,
    ),
    reviewDueCount: 2,
    progress: const [
      ProgressPoint(date: 'Mon', score: 49),
      ProgressPoint(date: 'Tue', score: 51),
      ProgressPoint(date: 'Wed', score: 52),
      ProgressPoint(date: 'Thu', score: 54),
      ProgressPoint(date: 'Fri', score: 56),
      ProgressPoint(date: 'Sat', score: 57),
      ProgressPoint(date: 'Today', score: 58),
    ],
    provider: const ProviderStatus(
      name: 'deterministic-fallback',
      configured: false,
    ),
  );

  static List<PracticeMode> get practice => const [
    PracticeMode(
      id: 'writing',
      title: 'Writing',
      description: 'Bug reports, PRs, commits and technical explanations.',
      minutes: 10,
      available: true,
    ),
    PracticeMode(
      id: 'speaking',
      title: 'Speaking',
      description: 'Explain code and technical decisions out loud.',
      minutes: 15,
      available: true,
    ),
    PracticeMode(
      id: 'roleplay',
      title: 'Roleplay',
      description: 'Practice one focused developer conversation.',
      minutes: 15,
      available: true,
    ),
    PracticeMode(
      id: 'system-design',
      title: 'System Design',
      description: 'Explain architecture, constraints and failure handling.',
      minutes: 20,
      available: true,
    ),
    PracticeMode(
      id: 'technical-interview',
      title: 'Technical Interview',
      description: 'Practice clarifying requirements and defending trade-offs.',
      minutes: 20,
      available: true,
    ),
    PracticeMode(
      id: 'reading',
      title: 'Reading & Listening',
      description: 'Work through technical content in context.',
      minutes: 10,
      available: true,
    ),
    PracticeMode(
      id: 'work-import',
      title: 'Learn From My Work',
      description: 'Turn an error, issue or document into a mission.',
      minutes: 10,
      available: true,
    ),
  ];

  static List<DiagnosticQuestion> get diagnosticQuestions => const [
    DiagnosticQuestion(
      id: 'demo-writing',
      skill: 'technical_writing',
      mode: 'writing',
      prompt: 'How comfortable are you writing a reproducible bug report?',
      options: [
        'I need a lot of support',
        'I can do this with a template',
        'I can do this independently',
        'I can coach another developer',
      ],
    ),
    DiagnosticQuestion(
      id: 'demo-speaking',
      skill: 'speaking',
      mode: 'speaking',
      prompt: 'How comfortable are you explaining a technical decision aloud?',
      options: [
        'I need a lot of support',
        'I can do this with a template',
        'I can do this independently',
        'I can coach another developer',
      ],
    ),
    DiagnosticQuestion(
      id: 'demo-meeting',
      skill: 'meeting',
      mode: 'speaking',
      prompt: 'How comfortable are you asking for clarification in a meeting?',
      options: [
        'I need a lot of support',
        'I can do this with a template',
        'I can do this independently',
        'I can coach another developer',
      ],
    ),
  ];

  static List<RoleplayScenario> get roleplayScenarios => const [
    RoleplayScenario(
      id: 'demo-incident',
      title: 'Explain an incident to a teammate',
      partnerRole: 'Senior engineer',
      level: 'B1',
      context: 'An API returns HTTP 500 for large uploads.',
      goal: 'Explain the evidence, user impact and safe next step.',
      type: 'incident',
    ),
    RoleplayScenario(
      id: 'demo-pr-review',
      title: 'Defend a pull request trade-off',
      partnerRole: 'Reviewer',
      level: 'B1',
      context: 'Your PR adds a retry for transient upstream failures.',
      goal: 'Explain why the retry is bounded and observable.',
      type: 'code-review',
    ),
    RoleplayScenario(
      id: 'demo-system-design',
      title: 'Lead a system design discussion',
      partnerRole: 'Staff engineer',
      level: 'B2',
      context: 'Design a queue for bursty notification traffic.',
      goal: 'Explain constraints, failure handling and trade-offs.',
      type: 'system-design',
    ),
    RoleplayScenario(
      id: 'demo-technical-interview',
      title: 'Technical interview: design an API',
      partnerRole: 'Interviewer',
      level: 'B2',
      context: 'Design an API that supports idempotent payment requests.',
      goal: 'Clarify requirements and defend one design decision.',
      type: 'technical-interview',
    ),
  ];

  static List<VocabularyItem> get vocabulary => const [
    VocabularyItem(
      id: 'demo-reproduce',
      term: 'reproduce',
      domain: 'debugging',
      level: 'B1',
      definition: 'make the same problem happen again',
      example: 'We can reproduce the 500 error with a 12 MB file.',
      mastery: 0.45,
      relatedTerms: ['root cause'],
    ),
    VocabularyItem(
      id: 'demo-root-cause',
      term: 'root cause',
      domain: 'debugging',
      level: 'B1',
      definition: 'the fundamental reason a problem occurs',
      example: 'The root cause is an unchecked upload limit.',
      mastery: 0.60,
      relatedTerms: ['reproduce'],
    ),
  ];

  static const UsageSummary usage = UsageSummary(
    month: 'demo',
    estimatedCost: 0,
    budgetVnd: 300000,
    budgetUsedPercent: 0,
  );

  static const SettingsData settings = SettingsData(
    aiProvider: 'deepseek',
    fastModel: 'deepseek-v4-flash',
    smartModel: 'deepseek-v4-pro',
    deepSeekConfigured: false,
    deepSeekStatus: 'not_configured',
    speechConfigured: false,
    pronunciationOn: false,
    monthlyBudgetVnd: 300000,
  );

  static List<ReviewItem> get review => const [
    ReviewItem(
      id: 'mistake-1',
      kind: 'mistake',
      prompt: 'Correct this sentence: I created bug report',
      answer: 'I created a bug report',
      context: 'technical writing',
      level: 'B1',
    ),
    ReviewItem(
      id: 'vocab-1',
      kind: 'vocabulary',
      prompt: 'Use “reproduce” in a technical sentence.',
      answer: 'We can reproduce the 500 error with a 12 MB file.',
      context: 'to make a problem happen again',
      level: 'B1',
    ),
  ];

  static ProgressData get progress => ProgressData(
    state: home.state,
    repeatedMistakes: 1,
    technicalScore: 48,
    trend: home.progress,
  );

  static const VocabularyGraph vocabularyGraph = VocabularyGraph(
    nodes: [
      VocabularyGraphNode(
        id: 'demo-reproduce',
        label: 'reproduce',
        mastery: 0.45,
      ),
      VocabularyGraphNode(
        id: 'demo-root-cause',
        label: 'root cause',
        mastery: 0.60,
      ),
    ],
    edges: [
      VocabularyGraphEdge(
        from: 'demo-reproduce',
        to: 'demo-root-cause',
        relation: 'related',
      ),
    ],
  );

  static const WeeklySpeakingAssessment
  weeklySpeaking = WeeklySpeakingAssessment(
    week: 'This week',
    sessions: 1,
    evaluated: 0,
    averageScore: 0,
    averageFluency: 0,
    averageProsody: 0,
    recommendation:
        'Complete one speaking assessment this week to unlock pronunciation and prosody feedback.',
  );

  static const AnalyticsSummary analytics = AnalyticsSummary(
    windowDays: 7,
    minutesLearned: 25,
    missionsCompleted: 1,
    missionsCreated: 2,
    repeatedMistakes: 1,
    technicalCommunicationScore: 51,
    vocabularyMastery: 0.52,
    weeklySpeaking: weeklySpeaking,
  );

  static const List<ProviderCheck> providerChecks = [
    ProviderCheck(provider: 'DeepSeek', configured: false, status: 'fallback'),
    ProviderCheck(
      provider: 'Groq Whisper',
      configured: false,
      status: 'not_configured',
    ),
    ProviderCheck(
      provider: 'Azure Pronunciation',
      configured: false,
      status: 'not_configured',
    ),
    ProviderCheck(
      provider: 'Azure Neural TTS',
      configured: false,
      status: 'not_configured',
    ),
  ];
}
