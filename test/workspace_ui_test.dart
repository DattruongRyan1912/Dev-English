import 'dart:async';

import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/app/workspace_shell.dart';
import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/components.dart';
import 'package:devenglish/src/screens/workspace_screen.dart';
import 'package:devenglish/src/theme.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:devenglish/src/workspace_demo.dart';
import 'package:devenglish/src/workspace_api.dart';
import 'package:devenglish/src/workspace_models.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

class _ImportWorkspaceApi extends WorkspaceApi {
  var importCalls = 0;

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(
        data: WorkspaceData.empty(displayName: 'Ryan'),
        conversationId: '',
      );

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<ImportedWorkspaceSource> importManualSource({
    required String name,
    required String content,
    required String idempotencyKey,
    String kind = 'manual',
    String uri = '',
    String mimeType = 'text/plain',
  }) async {
    importCalls++;
    return ImportedWorkspaceSource(
      source: KnowledgeSource(
        id: 'source-imported',
        title: name,
        kind: kind,
        updatedAt: 'Just now',
        uri: uri,
        evidence: [
          KnowledgeEvidence(
            id: 'evidence-imported',
            location: uri.isEmpty ? 'manual source' : uri,
            excerpt: content,
          ),
        ],
        claims: const [],
      ),
      revisionId: 'revision-imported',
      chunkId: 'chunk-imported',
    );
  }
}

class _QuickCaptureWorkspaceApi extends WorkspaceApi {
  var createTaskCalls = 0;

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(data: WorkspaceDemo.data, conversationId: '');

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<WorkspaceTask> createTask({
    required String projectId,
    required String title,
    String description = '',
    String priority = 'normal',
    required String idempotencyKey,
  }) async {
    createTaskCalls++;
    return WorkspaceTask(
      id: 'task-created',
      title: title,
      projectId: projectId,
      status: 'Todo',
      priority: priority,
      description: description,
    );
  }
}

class _DelayedWorkspaceApi extends WorkspaceApi {
  _DelayedWorkspaceApi(this.bootstrapCompleter);

  final Completer<WorkspaceSnapshot> bootstrapCompleter;

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) =>
      bootstrapCompleter.future;

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];
}

class _RetrievalStateWorkspaceApi extends WorkspaceApi {
  @override
  Future<List<WorkspaceSearchHit>> searchKnowledge(
    String query, {
    int limit = 20,
  }) async => const [
    WorkspaceSearchHit(
      id: 'hit-stale',
      sourceId: 'source-release',
      sourceName: 'Release notes',
      revisionId: 'revision-1',
      uri: 'manual://release-notes',
      excerpt: 'The indexed source is available through exact text fallback.',
      freshness: 'Stale',
      origin: 'Canonical',
      retrievalMode: 'Degraded',
    ),
  ];
}

class _ProviderFailureWorkspaceApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(
        data: WorkspaceData.empty(displayName: 'Ryan'),
        conversationId: '',
      );

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<WorkspaceAssistantResult> startConversation(String message) async {
    throw const ApiException(503, 'provider unavailable');
  }
}

class _FailedBootstrapWorkspaceApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async {
    throw const ApiException(503, 'workspace backend unavailable');
  }
}

class _ConflictWorkspaceApi extends WorkspaceApi {
  @override
  Future<WorkspaceProject> updateProject({
    required String projectId,
    String? name,
    String? description,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async {
    throw const ApiException(409, 'version conflict');
  }
}

class _HistoryWorkspaceApi extends WorkspaceApi {
  @override
  Future<List<WorkspaceHistoryEvent>> projectHistory(String projectId) async =>
      const [
        WorkspaceHistoryEvent(
          id: 'history-project-1',
          workspaceId: 'workspace-test',
          actorUserId: 'user-test',
          entityType: 'project',
          entityId: 'project-1',
          action: 'Created',
          fromVersion: 0,
          toVersion: 1,
          idempotencyKey: 'project-create-1',
          createdAt: null,
        ),
      ];
}

class _SyncWorkspaceApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(
        data: WorkspaceData.empty(displayName: 'Ryan'),
        conversationId: '',
      );

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<WorkspaceSyncResult> syncDrive({
    String cursor = '',
    int pageSize = 100,
  }) async => const WorkspaceSyncResult(
    seen: 4,
    upserted: 2,
    skipped: 2,
    nextCursor: 'cursor-2',
    hasMore: false,
  );
}

void main() {
  late AppController learningController;
  late WorkspaceController workspaceController;

  setUp(() {
    learningController = AppController();
    workspaceController = WorkspaceController();
  });

  tearDown(() {
    learningController.dispose();
    workspaceController.dispose();
  });

  Future<void> pumpWorkspace(WidgetTester tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: WorkspaceShell(
          controller: workspaceController,
          learningController: learningController,
          onPractice: () {},
          onReview: () {},
          onProgress: () {},
          onDiagnostic: () {},
          onRoleplay: () {},
          onCopilot: () {},
          onSpeaking: () {},
          onSettings: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('renders the four destinations and source-backed knowledge', (
    tester,
  ) async {
    await pumpWorkspace(tester);

    expect(find.text('Today'), findsWidgets);
    expect(find.text('Ask your assistant'), findsOneWidget);
    expect(find.byKey(const ValueKey('today-next-action')), findsOneWidget);
    expect(find.byKey(const ValueKey('today-quick-capture')), findsOneWidget);
    expect(find.text('New task'), findsOneWidget);
    expect(find.text('Record decision'), findsOneWidget);
    expect(find.text('Add source'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('workspace-voice-composer')),
      findsOneWidget,
    );
    expect(find.text('Hold to talk'), findsOneWidget);
    expect(find.text('DevEnglish'), findsOneWidget);

    await tester.tap(find.text('Work'));
    await tester.pumpAndSettle();
    expect(find.text('Projects'), findsOneWidget);
    expect(find.text('Open tasks'), findsOneWidget);
    expect(find.text('Dev-English'), findsWidgets);

    await tester.tap(find.byIcon(Icons.library_books_outlined));
    await tester.pumpAndSettle();
    expect(find.text('Connected sources'), findsOneWidget);
    expect(find.text('Demo preview'), findsWidgets);
    expect(find.text('DevEnglish V1 delivery plan'), findsWidgets);
  });

  testWidgets('Today quick capture creates a task through the workspace API', (
    tester,
  ) async {
    final api = _QuickCaptureWorkspaceApi();
    final controller = WorkspaceController(workspaceApi: api);
    addTearDown(controller.dispose);
    final appController = AppController();
    addTearDown(appController.dispose);
    await controller.load();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: WorkspaceShell(
          controller: controller,
          learningController: appController,
          onPractice: () {},
          onReview: () {},
          onProgress: () {},
          onDiagnostic: () {},
          onRoleplay: () {},
          onCopilot: () {},
          onSpeaking: () {},
          onSettings: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.byKey(const ValueKey('today-quick-task')));
    await tester.tap(find.byKey(const ValueKey('today-quick-task')));
    await tester.pumpAndSettle();
    expect(find.text('Quick capture task'), findsOneWidget);

    final taskField = find.byWidgetPredicate(
      (widget) => widget is TextField && widget.decoration?.labelText == 'Task',
    );
    expect(taskField, findsOneWidget);
    await tester.enterText(taskField, 'Write release notes');
    await tester.tap(find.text('Create task'));
    await tester.pumpAndSettle();

    expect(api.createTaskCalls, 1);
  });

  testWidgets('assistant keeps text, shows citations and preserves unknowns', (
    tester,
  ) async {
    await pumpWorkspace(tester);

    final composer = find.byKey(const ValueKey('assistant-composer'));
    await tester.enterText(composer, 'Show my current tasks');
    await tester.ensureVisible(find.byTooltip('Send message'));
    await tester.tap(find.byTooltip('Send message'));
    await tester.pump();

    expect(find.text('Show my current tasks'), findsOneWidget);
    expect(find.text('Checking the connected source...'), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 200));
    expect(
      find.text(
        'The current high-priority task is to finish the Today, Work and Knowledge workspace UI. '
        'The next safe step is to validate the text assistant flow and its citations before adding voice features.',
      ),
      findsOneWidget,
    );
    expect(find.text('Demo preview'), findsOneWidget);

    await tester.tap(find.text('Work'));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.today_outlined));
    await tester.pumpAndSettle();
    expect(
      find.text(
        'The current high-priority task is to finish the Today, Work and Knowledge workspace UI. '
        'The next safe step is to validate the text assistant flow and its citations before adding voice features.',
      ),
      findsOneWidget,
    );

    await tester.enterText(composer, 'What is unknown?');
    await tester.ensureVisible(find.byTooltip('Send message'));
    await tester.tap(find.byTooltip('Send message'));
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.textContaining('I cannot verify that'), findsOneWidget);
    expect(find.textContaining('Unknown:'), findsOneWidget);
  });

  testWidgets('knowledge search exposes an explicit empty state', (
    tester,
  ) async {
    await pumpWorkspace(tester);
    await tester.tap(find.byIcon(Icons.library_books_outlined));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('knowledge-search')), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('knowledge-search')),
      'not in the source',
    );
    await tester.pumpAndSettle();
    expect(find.text('No matching source'), findsOneWidget);
    expect(
      find.text('The assistant will keep an unsupported detail unknown.'),
      findsOneWidget,
    );
  });

  testWidgets('canonical bootstrap failure is visible and retryable', (
    tester,
  ) async {
    final controller = WorkspaceController(
      workspaceApi: _FailedBootstrapWorkspaceApi(),
    );
    addTearDown(controller.dispose);
    await controller.load();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: WorkspaceShell(
          controller: controller,
          learningController: learningController,
          onPractice: () {},
          onReview: () {},
          onProgress: () {},
          onDiagnostic: () {},
          onRoleplay: () {},
          onCopilot: () {},
          onSpeaking: () {},
          onSettings: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Workspace is temporarily unavailable'), findsOneWidget);
    expect(find.text('Retry connection'), findsOneWidget);
    expect(
      find.text('Backend đang gặp sự cố tạm thời. Hãy thử lại sau.'),
      findsOneWidget,
    );
    expect(find.text('workspace backend unavailable'), findsNothing);
    expect(find.text('Preview workspace'), findsNothing);
  });

  testWidgets(
    'compact navigation and legacy learning callbacks remain usable',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(390, 844));
      await pumpWorkspace(tester);

      expect(find.byType(NavigationBar), findsOneWidget);
      await tester.tap(find.text('Learning'));
      await tester.pumpAndSettle();
      expect(find.text('Choose a focused practice'), findsOneWidget);
      final learningHelp = find.textContaining('Need a little help?');
      expect(learningHelp, findsOneWidget);
      await tester.ensureVisible(learningHelp);
      await tester.tap(learningHelp);
      await tester.pumpAndSettle();
      expect(find.text('Giải thích ngắn'), findsOneWidget);
      expect(find.textContaining('The issue happens when'), findsOneWidget);

      var practiceCalls = 0;
      var reviewCalls = 0;
      var roleplayCalls = 0;
      var speakingCalls = 0;
      var copilotCalls = 0;
      var progressCalls = 0;
      var diagnosticCalls = 0;
      await tester.pumpWidget(
        MaterialApp(
          theme: buildAppTheme(),
          home: LearningWorkspaceScreen(
            controller: learningController,
            onPractice: () => practiceCalls++,
            onReview: () => reviewCalls++,
            onProgress: () => progressCalls++,
            onDiagnostic: () => diagnosticCalls++,
            onRoleplay: () => roleplayCalls++,
            onSpeaking: () => speakingCalls++,
            onCopilot: () => copilotCalls++,
          ),
        ),
      );
      await tester.pumpAndSettle();

      for (final label in [
        'Practice',
        'Review',
        'Roleplay',
        'Speaking',
        'English Copilot',
        'Progress',
        'Diagnostic',
      ]) {
        await tester.ensureVisible(find.text(label));
        await tester.tap(find.text(label));
      }
      expect(practiceCalls, 1);
      expect(reviewCalls, 1);
      expect(roleplayCalls, 1);
      expect(speakingCalls, 1);
      expect(copilotCalls, 1);
      expect(progressCalls, 1);
      expect(diagnosticCalls, 1);
    },
  );

  testWidgets('Today settings stays in the compact header', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    await pumpWorkspace(tester);

    final settings = find.byTooltip('Settings');
    expect(settings, findsOneWidget);
    expect(tester.getTopRight(settings).dx, greaterThan(330));
    expect(tester.getCenter(settings).dy, lessThan(150));
  });

  testWidgets('work and knowledge fit the compact viewport', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    await pumpWorkspace(tester);

    await tester.tap(find.text('Work'));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);

    await tester.tap(find.byIcon(Icons.library_books_outlined));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('workspace shell has no layout exception at release viewports', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    for (final viewport in const [
      Size(390, 844),
      Size(430, 932),
      Size(1440, 900),
    ]) {
      await tester.binding.setSurfaceSize(viewport);
      await pumpWorkspace(tester);
      expect(
        tester.takeException(),
        isNull,
        reason: 'workspace overflow at $viewport',
      );
      expect(find.text('Today'), findsWidgets);
    }
  });

  testWidgets('knowledge import requires an explicit review confirmation', (
    tester,
  ) async {
    final api = _ImportWorkspaceApi();
    final canonicalController = WorkspaceController(workspaceApi: api);
    addTearDown(canonicalController.dispose);
    await canonicalController.load();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: WorkspaceShell(
          controller: canonicalController,
          learningController: learningController,
          onPractice: () {},
          onReview: () {},
          onProgress: () {},
          onDiagnostic: () {},
          onRoleplay: () {},
          onCopilot: () {},
          onSpeaking: () {},
          onSettings: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.library_books_outlined));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Add source'));
    await tester.pumpAndSettle();

    expect(find.text('Draft knowledge source'), findsOneWidget);
    final fields = find.descendant(
      of: find.byType(AlertDialog).last,
      matching: find.byType(TextField),
    );
    await tester.enterText(fields.at(0), 'Release notes');
    await tester.enterText(fields.at(1), 'manual://release-notes');
    await tester.enterText(
      fields.at(2),
      'Deploy only after the smoke test passes.',
    );
    await tester.tap(find.text('Review import'));
    await tester.pumpAndSettle();

    expect(api.importCalls, 0);
    expect(find.text('Preview · no data written'), findsOneWidget);
    expect(find.text('Review source before import'), findsOneWidget);

    await tester.tap(find.text('Import verified source'));
    await tester.pumpAndSettle();
    expect(api.importCalls, 1);
  });

  testWidgets(
    'canonical loading state is explicit while bootstrap is pending',
    (tester) async {
      final bootstrap = Completer<WorkspaceSnapshot>();
      final controller = WorkspaceController(
        data: const WorkspaceData.empty(displayName: 'Ryan'),
        workspaceApi: _DelayedWorkspaceApi(bootstrap),
      );
      addTearDown(controller.dispose);

      await tester.pumpWidget(
        MaterialApp(
          theme: buildAppTheme(),
          home: Scaffold(
            body: TodayWorkspaceScreen(
              controller: controller,
              onSettings: () {},
            ),
          ),
        ),
      );
      final loading = controller.load();
      await tester.pump();
      expect(find.byKey(const ValueKey('workspace-loading')), findsOneWidget);
      expect(
        find.text('Refreshing canonical workspace data...'),
        findsOneWidget,
      );

      bootstrap.complete(
        const WorkspaceSnapshot(
          data: WorkspaceData.empty(displayName: 'Ryan'),
          conversationId: '',
        ),
      );
      await loading;
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('workspace-loading')), findsNothing);
    },
  );

  testWidgets('Today reflects canonical module capability state', (
    tester,
  ) async {
    const data = WorkspaceData(
      displayName: 'Ryan',
      origin: WorkspaceDataOrigin.canonical,
      projects: [],
      tasks: [],
      decisions: [],
      sources: [],
      initialConversation: [],
      capabilities: {
        'work': true,
        'knowledge': false,
        'assistant': true,
        'learning': false,
      },
    );
    final controller = WorkspaceController(data: data);
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: TodayWorkspaceScreen(controller: controller, onSettings: () {}),
        ),
      ),
    );

    expect(
      find.byKey(const ValueKey('workspace-capability-summary')),
      findsOneWidget,
    );
    expect(find.text('2/4 ready'), findsOneWidget);
    expect(find.text('Work ready'), findsOneWidget);
    expect(find.text('Knowledge off'), findsOneWidget);
  });

  testWidgets('knowledge search makes stale and degraded retrieval visible', (
    tester,
  ) async {
    final data = const WorkspaceData.empty(displayName: 'Ryan');
    final controller = WorkspaceController(
      data: data,
      workspaceApi: _RetrievalStateWorkspaceApi(),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: KnowledgeWorkspaceScreen(data: data, controller: controller),
        ),
      ),
    );
    await tester.enterText(
      find.byKey(const ValueKey('knowledge-search')),
      'release',
    );
    await tester.tap(find.byTooltip('Search knowledge'));
    await tester.pumpAndSettle();

    expect(find.text('Stale'), findsOneWidget);
    expect(find.text('Degraded · FTS fallback'), findsOneWidget);
    expect(find.text('Canonical'), findsOneWidget);
  });

  testWidgets('knowledge sync exposes cursor and read-only evidence', (
    tester,
  ) async {
    final api = _SyncWorkspaceApi();
    final controller = WorkspaceController(workspaceApi: api);
    addTearDown(controller.dispose);
    await controller.load();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: KnowledgeWorkspaceScreen(
            data: controller.data,
            controller: controller,
          ),
        ),
      ),
    );

    await tester.tap(find.text('Sync Drive'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('sync-status')), findsOneWidget);
    expect(find.text('Google Drive sync complete'), findsOneWidget);
    expect(
      find.text(
        '4 item(s) checked; 2 new revision(s) imported and 2 unchanged.',
      ),
      findsOneWidget,
    );
    expect(find.text('Cursor up to date'), findsOneWidget);
    expect(find.text('Read-only import'), findsOneWidget);
  });

  testWidgets('provider failure stays visible without a fallback answer', (
    tester,
  ) async {
    final controller = WorkspaceController(
      data: const WorkspaceData.empty(displayName: 'Ryan'),
      workspaceApi: _ProviderFailureWorkspaceApi(),
    );
    addTearDown(controller.dispose);
    await controller.load();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: TodayWorkspaceScreen(controller: controller, onSettings: () {}),
        ),
      ),
    );
    await tester.enterText(
      find.byKey(const ValueKey('assistant-composer')),
      'What is in my workspace?',
    );
    await tester.ensureVisible(find.byTooltip('Send message'));
    await tester.tap(find.byTooltip('Send message'));
    await tester.pumpAndSettle();

    expect(
      find.text('Backend đang gặp sự cố tạm thời. Hãy thử lại sau.'),
      findsWidgets,
    );
    expect(find.textContaining('provider unavailable'), findsNothing);
  });

  testWidgets('work conflict remains an explicit reload action', (
    tester,
  ) async {
    const project = WorkspaceProject(
      id: 'project-1',
      name: 'Dev-English',
      summary: 'Workspace project',
      status: 'Active',
      version: 2,
    );
    const data = WorkspaceData(
      displayName: 'Ryan',
      origin: WorkspaceDataOrigin.canonical,
      projects: [project],
      tasks: [],
      decisions: [],
      sources: [],
      initialConversation: [],
    );
    final controller = WorkspaceController(
      data: data,
      workspaceApi: _ConflictWorkspaceApi(),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: AnimatedBuilder(
          animation: controller,
          builder: (context, _) => Scaffold(
            body: WorkWorkspaceScreen(data: data, controller: controller),
          ),
        ),
      ),
    );
    await controller.updateProject(project: project, name: 'Changed');
    await tester.pump();
    expect(
      find.text(
        'Dữ liệu đã thay đổi ở nơi khác. Hãy tải lại workspace rồi thử lại.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('canonical work details expose versioned activity history', (
    tester,
  ) async {
    const project = WorkspaceProject(
      id: 'project-1',
      name: 'Dev-English',
      summary: 'Workspace project',
      status: 'Active',
      version: 1,
    );
    const data = WorkspaceData(
      displayName: 'Ryan',
      origin: WorkspaceDataOrigin.canonical,
      projects: [project],
      tasks: [],
      decisions: [],
      sources: [],
      initialConversation: [],
    );
    final controller = WorkspaceController(
      data: data,
      workspaceApi: _HistoryWorkspaceApi(),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: WorkWorkspaceScreen(data: data, controller: controller),
        ),
      ),
    );

    await tester.tap(find.byTooltip('Project actions'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Open details'));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('work-details-sheet')), findsOneWidget);
    expect(find.text('Dev-English'), findsWidgets);
    expect(find.text('Activity history'), findsOneWidget);
    expect(find.text('Created'), findsOneWidget);
    expect(find.text('Version 0 → 1 · Time unavailable'), findsOneWidget);
  });

  test('demo assistant only cites explicitly matched evidence', () async {
    const gateway = DemoAssistantGateway();
    final taskResponse = await gateway.ask(
      'Show my current tasks',
      WorkspaceDemo.data,
    );
    final unsupportedResponse = await gateway.ask(
      'What is the production release date?',
      WorkspaceDemo.data,
    );

    expect(taskResponse.evidence.single.location, 'Wave 3 / TASK-008');
    expect(
      taskResponse.evidence.single.excerpt,
      contains('current high-priority task'),
    );
    expect(unsupportedResponse.evidence, isEmpty);
    expect(unsupportedResponse.unknowns, isNotEmpty);
  });

  testWidgets(
    'learning support opens and hold-to-talk tracks press lifecycle',
    (tester) async {
      var starts = 0;
      var ends = 0;
      await tester.pumpWidget(
        MaterialApp(
          theme: buildAppTheme(),
          home: Scaffold(
            body: SingleChildScrollView(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                children: [
                  LearningSupportCard(
                    explanation: 'Giải thích ngắn.',
                    starter: 'The issue happens when ____.',
                    followUp: 'Name one verified detail.',
                  ),
                  HoldToTalkButton(
                    enabled: true,
                    recording: false,
                    onPressStart: () => starts++,
                    onPressEnd: () => ends++,
                    onPressCancel: () {},
                  ),
                ],
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Need a little help?'));
      await tester.pumpAndSettle();
      expect(find.text('Giải thích ngắn.'), findsOneWidget);

      final gesture = await tester.startGesture(
        tester.getCenter(find.byKey(const ValueKey('hold-to-talk'))),
      );
      await tester.pump();
      expect(starts, 1);
      await gesture.up();
      await tester.pump();
      expect(ends, 1);
    },
  );

  testWidgets('hold-to-talk supports keyboard activation', (tester) async {
    var recording = false;
    var starts = 0;
    var ends = 0;
    var cancels = 0;

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setState) => HoldToTalkButton(
              enabled: true,
              recording: recording,
              onPressStart: () => setState(() {
                starts++;
                recording = true;
              }),
              onPressEnd: () => setState(() {
                ends++;
                recording = false;
              }),
              onPressCancel: () => setState(() {
                cancels++;
                recording = false;
              }),
            ),
          ),
        ),
      ),
    );

    final focusNode = Focus.of(
      tester.element(find.byKey(const ValueKey('hold-to-talk'))),
      createDependency: false,
    );
    focusNode.requestFocus();
    await tester.pump();
    await tester.sendKeyDownEvent(LogicalKeyboardKey.space);
    await tester.pump();
    expect(starts, 1);
    expect(recording, isTrue);
    expect(cancels, 0);

    await tester.sendKeyUpEvent(LogicalKeyboardKey.space);
    await tester.pump();
    expect(ends, 1);
    expect(recording, isFalse);

    await tester.sendKeyDownEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    await tester.sendKeyUpEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(starts, 2);
    expect(ends, 2);
    expect(cancels, 0);
  });
}
