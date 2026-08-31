import 'package:devenglish/src/app/workspace_shell.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/components.dart';
import 'package:devenglish/src/screens/workspace_screen.dart';
import 'package:devenglish/src/theme.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:devenglish/src/workspace_models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

const _goldenViewports = <String, Size>{
  '390x844': Size(390, 844),
  '430x932': Size(430, 932),
  '1440x900': Size(1440, 900),
};

void main() {
  for (final entry in _goldenViewports.entries) {
    testWidgets('workspace surfaces match ${entry.key} goldens', (
      tester,
    ) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(entry.value);

      final learningController = AppController();
      final workspaceController = WorkspaceController();
      addTearDown(learningController.dispose);
      addTearDown(workspaceController.dispose);

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
      expect(tester.takeException(), isNull);

      await expectLater(
        find.byType(WorkspaceShell),
        matchesGoldenFile('goldens/workspace_${entry.key}_today.png'),
      );

      await _selectSurface(tester, 'Work');
      await expectLater(
        find.byType(WorkspaceShell),
        matchesGoldenFile('goldens/workspace_${entry.key}_work.png'),
      );

      await _selectSurface(tester, 'Knowledge');
      await expectLater(
        find.byType(WorkspaceShell),
        matchesGoldenFile('goldens/workspace_${entry.key}_knowledge.png'),
      );

      await _selectSurface(tester, 'Learning');
      await expectLater(
        find.byType(WorkspaceShell),
        matchesGoldenFile('goldens/workspace_${entry.key}_learning.png'),
      );
      expect(tester.takeException(), isNull);
    });

    testWidgets('assistant evidence surface matches ${entry.key} golden', (
      tester,
    ) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(entry.value);

      await tester.pumpWidget(
        MaterialApp(
          theme: buildAppTheme(),
          home: Scaffold(
            body: SafeArea(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(AppSpacing.page),
                child: Builder(
                  builder: (context) => Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const EyebrowLabel('DevEnglish / Assistant'),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        'Conversation',
                        style: Theme.of(context).textTheme.displaySmall,
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      const Text(
                        'A text-first answer with visible evidence and an explicit unknown boundary.',
                      ),
                      const SizedBox(height: AppSpacing.xl),
                      AssistantTurnBubble(
                        turn: AssistantTurn(
                          id: 'golden-grounded',
                          role: 'assistant',
                          content:
                              'The next safe step is to verify the current task against the connected project source.',
                          citations: const [
                            AssistantCitation(
                              sourceId: 'source-1',
                              sourceTitle: 'Release plan',
                              location: 'TASK-008 / acceptance',
                              excerpt:
                                  'The assistant should keep the answer tied to a verified source excerpt.',
                              origin: WorkspaceDataOrigin.canonical,
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: AppSpacing.md),
                      const AssistantTurnBubble(
                        turn: AssistantTurn(
                          id: 'golden-unknown',
                          role: 'assistant',
                          content:
                              'I cannot verify the requested release date from the connected sources yet.',
                          unknowns: [
                            'The release date is not present in the connected source.',
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      await expectLater(
        find.byType(Scaffold),
        matchesGoldenFile('goldens/workspace_${entry.key}_assistant.png'),
      );
    });
  }
}

Future<void> _selectSurface(WidgetTester tester, String label) async {
  final destination = find.text(label);
  expect(destination, findsWidgets);
  await tester.tap(destination.first);
  await tester.pumpAndSettle();
}
