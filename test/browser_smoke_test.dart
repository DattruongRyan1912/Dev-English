import 'package:devenglish/main.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

void main() {
  if (const bool.fromEnvironment('DEVENGLISH_BROWSER_SMOKE')) {
    IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  }

  testWidgets(
    'Chrome smoke loads backend state and navigates the four primary destinations',
    (tester) async {
      final controller = AppController();
      addTearDown(controller.dispose);

      await tester.pumpWidget(DevEnglishApp(controller: controller));
      for (var attempt = 0; attempt < 300 && controller.loading; attempt++) {
        await tester.pump(const Duration(milliseconds: 100));
      }
      await tester.pump();

      expect(
        controller.usingDemo,
        isFalse,
        reason: 'The browser smoke must fail when the backend is unavailable.',
      );
      expect(
        controller.home.mission.title,
        'Explain a technical problem clearly',
      );
      expect(controller.home.mission.id, startsWith('mission-generated-'));
      expect(find.text("Today's mission"), findsOneWidget);
      expect(find.text(controller.home.mission.title), findsOneWidget);

      await tester.tap(find.text('Practice'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Choose one clear task. Keep the session moving.'),
        findsOneWidget,
      );
      expect(find.text(controller.practice.first.title), findsOneWidget);

      await tester.tap(find.text('Review'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Due items first. Try before you reveal the answer.'),
        findsOneWidget,
      );
      if (controller.review.isEmpty) {
        expect(find.text('You are up to date'), findsOneWidget);
      } else {
        expect(find.text(controller.review.first.prompt), findsOneWidget);
      }

      await tester.tap(find.text('Progress'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Trends that help you choose the next useful practice.'),
        findsOneWidget,
      );
      expect(
        find.text(controller.progress.state.skills.first.label),
        findsOneWidget,
      );
    },
    skip: !const bool.fromEnvironment('DEVENGLISH_BROWSER_SMOKE'),
  );
}
