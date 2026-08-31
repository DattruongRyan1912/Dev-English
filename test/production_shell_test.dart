import 'dart:async';

import 'package:devenglish/main.dart';
import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/demo_data.dart';
import 'package:devenglish/src/models.dart';
import 'package:devenglish/src/screens/practice_screen.dart';
import 'package:devenglish/src/workspace_api.dart';
import 'package:devenglish/src/workspace_demo.dart';
import 'package:devenglish/src/workspace_models.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const _isProduction =
    String.fromEnvironment('DEVENGLISH_ENV', defaultValue: 'development') ==
    'production';

class _AuthenticatedApi extends DevEnglishApi {
  _AuthenticatedApi() : super(baseUrl: 'http://127.0.0.1:1');

  @override
  Future<AuthUser> currentUser() async =>
      const AuthUser(id: 'user-1', displayName: 'Ryan', cefr: 'B1');

  @override
  Future<HomeData> home() async => DemoData.home;

  @override
  Future<List<PracticeMode>> practice() async => DemoData.practice;

  @override
  Future<List<ReviewItem>> reviewDue() async => DemoData.review;

  @override
  Future<ProgressData> progress() async => DemoData.progress;

  @override
  Future<DiagnosticResult> diagnosticResult() async =>
      throw StateError('diagnostic fixture is not needed for this shell test');

  @override
  Future<AnalyticsSummary> analytics() async => DemoData.analytics;

  @override
  Future<WeeklySpeakingAssessment> weeklySpeaking() async =>
      DemoData.weeklySpeaking;
}

class _CanonicalWorkspaceApi extends WorkspaceApi {
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
}

class _AuthOnlyApi extends DevEnglishApi {
  _AuthOnlyApi() : super(baseUrl: 'http://127.0.0.1:1');

  @override
  Future<AuthUser> currentUser() async =>
      const AuthUser(id: 'user-1', displayName: 'Ryan', cefr: 'B1');
}

class _CountingAuthenticatedApi extends _AuthenticatedApi {
  var homeCalls = 0;

  @override
  Future<HomeData> home() async {
    homeCalls++;
    await Future<void>.delayed(const Duration(milliseconds: 10));
    return DemoData.home;
  }
}

class _DeferredLegacyApi extends _AuthenticatedApi {
  final homeCompleter = Completer<HomeData>();

  @override
  Future<HomeData> home() => homeCompleter.future;

  @override
  Future<void> logout() async {}
}

class _DeferredWorkspaceApi extends _CanonicalWorkspaceApi {
  final bootstrapCompleter = Completer<WorkspaceSnapshot>();

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) =>
      bootstrapCompleter.future;
}

class _DeferredConversationWorkspaceApi extends _CanonicalWorkspaceApi {
  final startConversationCompleter = Completer<WorkspaceAssistantResult>();
  final listConversationsCompleter =
      Completer<List<WorkspaceConversationSummary>>();
  final getConversationCompleter = Completer<WorkspaceConversation>();

  @override
  Future<WorkspaceAssistantResult> startConversation(String message) =>
      startConversationCompleter.future;

  @override
  Future<List<WorkspaceConversationSummary>> listConversations({
    int limit = 20,
  }) => listConversationsCompleter.future;

  @override
  Future<WorkspaceConversation> getConversation(String conversationId) =>
      getConversationCompleter.future;
}

class _DeferredMutationWorkspaceApi extends _CanonicalWorkspaceApi {
  final createProjectCompleter = Completer<WorkspaceProject>();
  var bootstrapCalls = 0;

  @override
  Future<WorkspaceProject> createProject({
    required String name,
    String description = '',
    required String idempotencyKey,
  }) => createProjectCompleter.future;

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) {
    bootstrapCalls++;
    return super.bootstrap(includeTrashed: includeTrashed);
  }
}

void main() {
  testWidgets(
    'successful authenticated production bootstrap opens the new canonical shell',
    (tester) async {
      final controller = AppController(api: _AuthenticatedApi());
      final workspaceController = WorkspaceController(
        workspaceApi: _CanonicalWorkspaceApi(),
      );
      addTearDown(controller.dispose);
      addTearDown(workspaceController.dispose);

      await tester.pumpWidget(
        DevEnglishApp(
          controller: controller,
          workspaceController: workspaceController,
        ),
      );
      await tester.pumpAndSettle();

      expect(controller.authenticated, isTrue);
      expect(controller.usingDemo, isFalse);
      expect(workspaceController.canonicalLoaded, isTrue);
      expect(find.text('Ask your assistant'), findsOneWidget);
      expect(find.text('Today'), findsWidgets);
      expect(find.textContaining('Local preview:'), findsNothing);
      expect(find.text('Canonical workspace'), findsWidgets);

      // This is the regression guard for the old authenticated-production
      // path: authentication must expose the complete canonical shell, not a
      // single HomeScreen replacement.
      await _tapCanonicalDestination(tester, 'Work', Icons.work_outline);
      expect(find.text('Projects'), findsOneWidget);
      expect(find.text('Open tasks'), findsOneWidget);

      await _tapCanonicalDestination(
        tester,
        'Knowledge',
        Icons.library_books_outlined,
      );
      expect(find.text('Connected sources'), findsOneWidget);

      await _tapCanonicalDestination(tester, 'Learning', Icons.school_outlined);
      expect(find.text('Choose a focused practice'), findsOneWidget);

      await _tapCanonicalDestination(tester, 'Today', Icons.today_outlined);
      expect(find.text('Ask your assistant'), findsOneWidget);
    },
    skip: !_isProduction,
  );

  testWidgets('new shell does not require legacy learning endpoints', (
    tester,
  ) async {
    final controller = AppController(api: _AuthOnlyApi(), loadLegacy: false);
    final workspaceController = WorkspaceController(
      workspaceApi: _CanonicalWorkspaceApi(),
    );
    addTearDown(controller.dispose);
    addTearDown(workspaceController.dispose);

    await tester.pumpWidget(
      DevEnglishApp(
        controller: controller,
        workspaceController: workspaceController,
      ),
    );
    await tester.pumpAndSettle();

    expect(controller.legacyLoaded, isFalse);
    expect(workspaceController.canonicalLoaded, isTrue);
    expect(find.text('Ask your assistant'), findsOneWidget);

    await tester.tap(find.byTooltip('Settings'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('settings-unavailable')), findsOneWidget);
    expect(
      find.text('Settings data is temporarily unavailable.'),
      findsOneWidget,
    );
    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Learning'));
    await tester.pumpAndSettle();
    expect(find.text('Workflow overlay ready'), findsOneWidget);

    await tester.ensureVisible(find.text('Practice'));
    await tester.tap(find.text('Practice'));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('legacy-learning-unavailable')),
      findsOneWidget,
    );
    expect(
      find.text('Learning data is temporarily unavailable.'),
      findsOneWidget,
    );
    expect(find.text('Explain a technical problem clearly'), findsNothing);
  });

  testWidgets('new shell lazy-loads real learning data before opening a tool', (
    tester,
  ) async {
    final controller = AppController(
      api: _AuthenticatedApi(),
      loadLegacy: false,
    );
    final workspaceController = WorkspaceController(
      workspaceApi: _CanonicalWorkspaceApi(),
    );
    addTearDown(controller.dispose);
    addTearDown(workspaceController.dispose);

    await tester.pumpWidget(
      DevEnglishApp(
        controller: controller,
        workspaceController: workspaceController,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Learning'));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('Practice'));
    await tester.tap(find.text('Practice'));
    await tester.pumpAndSettle();

    expect(find.byType(PracticeScreen), findsOneWidget);
    expect(find.text('Writing'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('legacy-learning-unavailable')),
      findsNothing,
    );
    expect(controller.legacyLoaded, isTrue);
  });

  test('legacy learning load is shared by concurrent callers', () async {
    final api = _CountingAuthenticatedApi();
    final controller = AppController(api: api, loadLegacy: false);
    addTearDown(controller.dispose);

    final results = await Future.wait([
      controller.ensureLegacyLearningLoaded(),
      controller.ensureLegacyLearningLoaded(),
    ]);

    expect(results, [true, true]);
    expect(api.homeCalls, 1);
    expect(controller.legacyLoaded, isTrue);
  });

  test('logout clears user-scoped learning and workspace state', () async {
    final learningController = AppController(
      api: _AuthenticatedApi(),
      loadLegacy: false,
    );
    final workspaceController = WorkspaceController(
      data: WorkspaceDemo.data,
      workspaceApi: _CanonicalWorkspaceApi(),
    );
    addTearDown(learningController.dispose);
    addTearDown(workspaceController.dispose);

    await learningController.ensureLegacyLearningLoaded();
    expect(learningController.legacyLoaded, isTrue);
    expect(workspaceController.data.projects, isNotEmpty);

    await learningController.logout();
    workspaceController.resetForLogout();

    expect(learningController.legacyLoaded, isFalse);
    expect(learningController.settingsLoaded, isFalse);
    expect(workspaceController.canonicalLoaded, isFalse);
    expect(workspaceController.data.projects, isEmpty);
    expect(workspaceController.conversation, isEmpty);
  });

  test(
    'late legacy learning response cannot restore state after logout',
    () async {
      final api = _DeferredLegacyApi();
      final controller = AppController(api: api, loadLegacy: false);
      addTearDown(controller.dispose);

      final loading = controller.ensureLegacyLearningLoaded();
      await Future<void>.delayed(Duration.zero);
      await controller.logout();
      final demoHome = DemoData.home;
      api.homeCompleter.complete(
        HomeData(
          name: 'stale-user',
          state: demoHome.state,
          mission: demoHome.mission,
          reviewDueCount: demoHome.reviewDueCount,
          progress: demoHome.progress,
          provider: demoHome.provider,
        ),
      );

      expect(await loading, isFalse);
      expect(controller.legacyLoaded, isFalse);
      expect(controller.home.name, 'Developer');
    },
  );

  test(
    'late workspace bootstrap cannot restore state after logout reset',
    () async {
      final api = _DeferredWorkspaceApi();
      final controller = WorkspaceController(
        data: WorkspaceDemo.data,
        workspaceApi: api,
      );
      addTearDown(controller.dispose);

      final loading = controller.load();
      await Future<void>.delayed(Duration.zero);
      controller.resetForLogout();
      api.bootstrapCompleter.complete(
        const WorkspaceSnapshot(
          data: WorkspaceDemo.data,
          conversationId: 'stale-conversation',
        ),
      );

      await loading;
      expect(controller.canonicalLoaded, isFalse);
      expect(controller.data.projects, isEmpty);
      expect(controller.conversation, isEmpty);
      expect(controller.conversationId, isEmpty);
    },
  );

  test(
    'late assistant response cannot restore transcript after logout reset',
    () async {
      final api = _DeferredConversationWorkspaceApi();
      final controller = WorkspaceController(
        data: WorkspaceDemo.data,
        workspaceApi: api,
      );
      addTearDown(controller.dispose);

      final asking = controller.ask('stale prompt');
      await Future<void>.delayed(Duration.zero);
      controller.resetForLogout();
      api.startConversationCompleter.complete(
        const WorkspaceAssistantResult(
          conversationId: 'stale-conversation',
          response: AssistantResponse(
            answer: 'stale answer',
            evidence: [],
            unknowns: [],
            staleSources: [],
            suggestedActions: [],
            actionReceipts: [],
          ),
        ),
      );

      await asking;
      expect(controller.conversation, isEmpty);
      expect(controller.conversationHistory, isEmpty);
      expect(controller.conversationId, isEmpty);
      expect(controller.sending, isFalse);
    },
  );

  test(
    'late conversation history cannot restore user-scoped state after reset',
    () async {
      final api = _DeferredConversationWorkspaceApi();
      final controller = WorkspaceController(
        data: WorkspaceDemo.data,
        workspaceApi: api,
      );
      addTearDown(controller.dispose);

      final asking = controller.ask('wait for history');
      await Future<void>.delayed(Duration.zero);
      api.startConversationCompleter.complete(
        const WorkspaceAssistantResult(
          conversationId: 'stale-conversation',
          response: AssistantResponse(
            answer: 'stale answer',
            evidence: [],
            unknowns: [],
            staleSources: [],
            suggestedActions: [],
            actionReceipts: [],
          ),
        ),
      );
      await Future<void>.delayed(Duration.zero);
      controller.resetForLogout();
      api.listConversationsCompleter.complete(const [
        WorkspaceConversationSummary(
          id: 'stale-conversation',
          title: 'Stale conversation',
          status: 'active',
          messageCount: 1,
        ),
      ]);

      await asking;
      expect(controller.conversation, isEmpty);
      expect(controller.conversationHistory, isEmpty);
      expect(controller.conversationId, isEmpty);
    },
  );

  test(
    'late opened conversation cannot restore transcript after reset',
    () async {
      final api = _DeferredConversationWorkspaceApi();
      final controller = WorkspaceController(
        data: WorkspaceDemo.data,
        workspaceApi: api,
      );
      addTearDown(controller.dispose);

      final opening = controller.openConversation('stale-conversation');
      await Future<void>.delayed(Duration.zero);
      controller.resetForLogout();
      api.getConversationCompleter.complete(
        const WorkspaceConversation(
          id: 'stale-conversation',
          title: 'Stale conversation',
          status: 'active',
          messages: [
            AssistantTurn(
              id: 'stale-message',
              role: 'assistant',
              content: 'stale message',
            ),
          ],
        ),
      );

      await opening;
      expect(controller.conversation, isEmpty);
      expect(controller.conversationId, isEmpty);
      expect(controller.conversationLoading, isFalse);
    },
  );

  test('late mutation cannot trigger a bootstrap after logout reset', () async {
    final api = _DeferredMutationWorkspaceApi();
    final controller = WorkspaceController(
      data: WorkspaceDemo.data,
      workspaceApi: api,
    );
    addTearDown(controller.dispose);

    final creating = controller.createProject(name: 'stale project');
    await Future<void>.delayed(Duration.zero);
    controller.resetForLogout();
    api.createProjectCompleter.complete(
      const WorkspaceProject(
        id: 'stale-project',
        name: 'Stale project',
        summary: '',
        status: 'active',
      ),
    );

    await creating;
    expect(api.bootstrapCalls, 0);
    expect(controller.data.projects, isEmpty);
    expect(controller.canonicalLoaded, isFalse);
  });
}

Future<void> _tapCanonicalDestination(
  WidgetTester tester,
  String label,
  IconData icon,
) async {
  final iconFinder = find.byIcon(icon);
  if (iconFinder.evaluate().isNotEmpty) {
    await tester.tap(iconFinder.first);
  } else {
    await tester.tap(find.bySemanticsLabel(label).first);
  }
  await tester.pumpAndSettle();
}
