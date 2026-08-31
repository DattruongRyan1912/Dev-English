import 'package:devenglish/main.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/screens/speaking_screen.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders the workspace shell with text assistant', (
    tester,
  ) async {
    // Keep the smoke deterministic: the production app bootstraps its
    // canonical workspace over HTTP, while this test only verifies shell
    // composition and should not depend on Flutter's blocked test client.
    final workspaceController = WorkspaceController();
    addTearDown(workspaceController.dispose);
    await tester.pumpWidget(
      DevEnglishApp(workspaceController: workspaceController),
    );
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

  test('wrapPcm16AsWav writes a PCM WAV header', () {
    final wav = wrapPcm16AsWav(List<int>.filled(4, 0), sampleRate: 16000);
    expect(wav.length, 48);
    expect(String.fromCharCodes(wav.sublist(0, 4)), 'RIFF');
    expect(String.fromCharCodes(wav.sublist(8, 12)), 'WAVE');
    expect(wav[22], 1);
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
