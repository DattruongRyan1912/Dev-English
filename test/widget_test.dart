import 'package:flutter/material.dart';
import 'package:devenglish/main.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/screens/speaking_screen.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders the workspace shell with text assistant', (
    tester,
  ) async {
    await tester.pumpWidget(const DevEnglishApp());
    await tester.pump(const Duration(milliseconds: 500));
    if (const String.fromEnvironment(
          'DEVENGLISH_ENV',
          defaultValue: 'development',
        ) ==
        'production') {
      expect(find.text('Sign in to DevEnglish'), findsOneWidget);
      return;
    }
    expect(find.text('Today'), findsWidgets);
    expect(find.text('Ask your assistant'), findsOneWidget);
    expect(find.text('What should I do next?'), findsOneWidget);
  });

  testWidgets('renders speaking controls and manual fallback', (tester) async {
    final controller = AppController();
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(home: SpeakingScreen(controller: controller)),
    );
    await tester.pump();

    expect(find.text('Speaking'), findsOneWidget);
    expect(find.text('Speech state: Ready'), findsOneWidget);
    expect(find.text('Start recording'), findsOneWidget);
    expect(find.text('Transcript / manual fallback'), findsOneWidget);
    expect(find.text('Save transcript'), findsOneWidget);
  });
}
