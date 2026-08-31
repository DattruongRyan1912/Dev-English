import 'package:devenglish/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

void main() {
  if (const bool.fromEnvironment('DEVENGLISH_BROWSER_SMOKE')) {
    IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  }

  testWidgets(
    'Chrome smoke loads the canonical workspace and navigates its four destinations',
    (tester) async {
      // Do not inject the legacy AppController here. The smoke must exercise
      // the production app composition, including auth, bootstrap and the
      // canonical workspace API.
      await tester.pumpWidget(const DevEnglishApp());
      for (var attempt = 0; attempt < 300; attempt++) {
        await tester.pump(const Duration(milliseconds: 100));
        if (find.text('Canonical workspace').evaluate().isNotEmpty ||
            find.text('Workspace unavailable').evaluate().isNotEmpty) {
          break;
        }
      }
      await tester.pump();

      expect(find.text('Canonical workspace'), findsOneWidget);
      expect(find.text('Ask your assistant'), findsOneWidget);

      Future<void> selectDestination(String label, IconData icon) async {
        final iconFinder = find.byIcon(icon);
        if (iconFinder.evaluate().isNotEmpty) {
          await tester.tap(iconFinder.first);
        } else {
          await tester.tap(find.text(label).last);
        }
        await tester.pump(const Duration(milliseconds: 300));
      }

      await selectDestination('Work', Icons.work_outline);
      expect(find.text('Projects'), findsOneWidget);
      expect(find.text('Open tasks'), findsOneWidget);

      await selectDestination('Knowledge', Icons.library_books_outlined);
      expect(find.text('Connected sources'), findsOneWidget);
      expect(find.byKey(const ValueKey('knowledge-search')), findsOneWidget);

      await selectDestination('Learning', Icons.school_outlined);
      expect(find.text('Choose a focused practice'), findsOneWidget);
      expect(
        find.text(
          'Learning observations stay separate from canonical work data.',
        ),
        findsOneWidget,
      );

      await selectDestination('Today', Icons.today_outlined);
      expect(find.text('Ask your assistant'), findsOneWidget);
    },
    skip: !const bool.fromEnvironment('DEVENGLISH_BROWSER_SMOKE'),
  );
}
