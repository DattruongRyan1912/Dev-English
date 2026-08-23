import 'package:devenglish/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'demo navigation smoke covers the four primary destinations',
    (tester) async {
      await tester.pumpWidget(const DevEnglishApp());
      await tester.pump(const Duration(seconds: 6));

      expect(find.text("Today's mission"), findsOneWidget);

      await tester.tap(find.text('Practice'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Choose one clear task. Keep the session moving.'),
        findsOneWidget,
      );

      await tester.tap(find.text('Review'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Due items first. Try before you reveal the answer.'),
        findsOneWidget,
      );

      await tester.tap(find.text('Progress'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(
        find.text('Trends that help you choose the next useful practice.'),
        findsOneWidget,
      );
    },
    skip:
        const String.fromEnvironment(
          'DEVENGLISH_ENV',
          defaultValue: 'development',
        ) ==
        'production',
  );
}
