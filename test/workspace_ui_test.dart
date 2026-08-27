import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/components.dart';
import 'package:devenglish/src/screens/workspace_screen.dart';
import 'package:devenglish/src/theme.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:devenglish/src/workspace_demo.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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
    expect(find.text('DevEnglish'), findsOneWidget);

    await tester.tap(find.text('Work'));
    await tester.pumpAndSettle();
    expect(find.text('Projects'), findsOneWidget);
    expect(find.text('Open tasks'), findsOneWidget);
    expect(find.text('Dev-English'), findsOneWidget);

    await tester.tap(find.byIcon(Icons.library_books_outlined));
    await tester.pumpAndSettle();
    expect(find.text('Connected sources'), findsOneWidget);
    expect(find.text('Demo preview'), findsWidgets);
    expect(find.text('DevEnglish V1 delivery plan'), findsWidgets);
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
      expect(find.text('Need a little help?'), findsOneWidget);
      await tester.ensureVisible(find.text('Need a little help?'));
      await tester.tap(find.text('Need a little help?'));
      await tester.pumpAndSettle();
      expect(find.text('Giải thích ngắn'), findsOneWidget);
      expect(find.textContaining('The issue happens when'), findsOneWidget);

      var practiceCalls = 0;
      var reviewCalls = 0;
      var roleplayCalls = 0;
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
            onCopilot: () => copilotCalls++,
          ),
        ),
      );
      await tester.pumpAndSettle();

      for (final label in [
        'Practice',
        'Review',
        'Roleplay',
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
      expect(copilotCalls, 1);
      expect(progressCalls, 1);
      expect(diagnosticCalls, 1);
    },
  );

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
}
