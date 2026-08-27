import 'package:devenglish/main.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:flutter/material.dart';
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
      expect(find.text('Today'), findsWidgets);
      expect(find.text('Ask your assistant'), findsOneWidget);

      await tester.tap(find.text('Learning'));
      await tester.pumpAndSettle();
      expect(find.text('Choose a focused practice'), findsOneWidget);

      await tester.tap(find.text('Work'));
      await tester.pumpAndSettle();
      expect(find.text('Projects'), findsOneWidget);
      expect(find.text('Open tasks'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.library_books_outlined));
      await tester.pumpAndSettle();
      expect(find.text('Connected sources'), findsOneWidget);

      await tester.tap(find.text('Learning'));
      await tester.pumpAndSettle();
      expect(find.text('Choose a focused practice'), findsOneWidget);
    },
    skip: !const bool.fromEnvironment('DEVENGLISH_BROWSER_SMOKE'),
  );
}
