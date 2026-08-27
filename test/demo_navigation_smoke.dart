import 'package:devenglish/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'workspace navigation smoke covers the four primary destinations',
    (tester) async {
      await tester.pumpWidget(const DevEnglishApp());
      await tester.pump(const Duration(seconds: 6));

      expect(find.text('Today'), findsWidgets);

      await tester.tap(find.text('Work'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.text('Open tasks'), findsOneWidget);

      await tester.tap(find.text('Knowledge'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.text('Connected sources'), findsOneWidget);

      await tester.tap(find.text('Learning'));
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.text('Current English layer'), findsOneWidget);
    },
    skip:
        const String.fromEnvironment(
          'DEVENGLISH_ENV',
          defaultValue: 'development',
        ) ==
        'production',
  );
}
