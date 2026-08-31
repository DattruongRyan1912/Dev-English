import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/components.dart';
import 'package:devenglish/src/features/workspace/workspace.dart';
import 'package:devenglish/src/theme.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart';
import 'package:http/testing.dart';
import 'package:record/record.dart';

class _FakeWorkspaceRecorder implements WorkspaceAudioRecorder {
  _FakeWorkspaceRecorder({
    this.permission = true,
    this.hangOnStop = false,
    this.startDelay = Duration.zero,
  });

  final bool permission;
  final bool hangOnStop;
  final Duration startDelay;
  StreamController<Uint8List>? _chunks;
  int startCalls = 0;
  int stopCalls = 0;
  bool disposed = false;

  @override
  Future<bool> hasPermission() async => permission;

  @override
  Future<Stream<Uint8List>> startStream(RecordConfig config) async {
    startCalls++;
    if (startDelay > Duration.zero) {
      await Future<void>.delayed(startDelay);
    }
    final chunks = StreamController<Uint8List>(sync: true);
    _chunks = chunks;
    return chunks.stream;
  }

  @override
  Future<void> stop() async {
    stopCalls++;
    if (hangOnStop) return Completer<void>().future;
    if (_chunks == null) return;
    // Let the production code own subscription cancellation. Closing the
    // controller here would make its close future wait on that cancellation.
    await Future<void>.delayed(Duration.zero);
  }

  @override
  Future<void> dispose() async {
    disposed = true;
    final chunks = _chunks;
    if (chunks != null && !chunks.isClosed) await chunks.close();
  }

  void emit(List<int> bytes) {
    final chunks = _chunks;
    if (chunks != null && !chunks.isClosed) {
      chunks.add(Uint8List.fromList(bytes));
    }
  }

  void fail(Object error) {
    final chunks = _chunks;
    if (chunks != null && !chunks.isClosed) chunks.addError(error);
  }
}

Future<void> _pumpVoiceScreen(
  WidgetTester tester, {
  required AppController speechController,
  required WorkspaceAudioRecorder recorder,
  Duration voiceStartupTimeout = const Duration(seconds: 10),
  Duration voiceStopTimeout = const Duration(seconds: 10),
}) async {
  final workspaceController = WorkspaceController();
  addTearDown(workspaceController.dispose);
  await tester.pumpWidget(
    MaterialApp(
      theme: buildAppTheme(),
      home: Scaffold(
        body: TodayWorkspaceScreen(
          controller: workspaceController,
          onSettings: () {},
          speechController: speechController,
          voiceRecorderFactory: () => recorder,
          voiceStartupTimeout: voiceStartupTimeout,
          voiceStopTimeout: voiceStopTimeout,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

AppController _speechController({required int Function() onTranscribe}) {
  return AppController(
    api: DevEnglishApi(
      client: MockClient((request) async {
        if (request.method == 'POST' &&
            request.url.path == '/api/v1/speaking/transcribe') {
          onTranscribe();
          return Response(
            jsonEncode({
              'id': 'speech-1',
              'status': 'transcribed',
              'transcript': 'I checked the logs.',
            }),
            200,
          );
        }
        return Response('{}', 404);
      }),
    ),
  );
}

void main() {
  testWidgets('workspace voice keeps an editable transcript after release', (
    tester,
  ) async {
    var transcribeCalls = 0;
    final speechController = _speechController(
      onTranscribe: () => transcribeCalls++,
    );
    final recorder = _FakeWorkspaceRecorder();
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
    );

    final button = find.byKey(const ValueKey('hold-to-talk'));
    await tester.ensureVisible(button);
    await tester.pumpAndSettle();
    final gesture = await tester.startGesture(tester.getCenter(button));
    await tester.pumpAndSettle();
    expect(find.text('Listening... release to transcribe.'), findsOneWidget);
    recorder.emit([1, 2, 3, 4]);
    await tester.pump();
    await gesture.up();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pumpAndSettle();

    expect(recorder.startCalls, 1);
    expect(transcribeCalls, 1);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('assistant-composer')))
          .controller
          ?.text,
      'I checked the logs.',
    );
    expect(find.textContaining('Transcript ready'), findsOneWidget);
  });

  testWidgets('workspace stream failure is terminal and never calls STT', (
    tester,
  ) async {
    var transcribeCalls = 0;
    final speechController = _speechController(
      onTranscribe: () => transcribeCalls++,
    );
    final recorder = _FakeWorkspaceRecorder();
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
    );

    final button = find.byKey(const ValueKey('hold-to-talk'));
    await tester.ensureVisible(button);
    await tester.pumpAndSettle();
    final gesture = await tester.startGesture(tester.getCenter(button));
    await tester.pumpAndSettle();
    recorder.emit([1, 2, 3]);
    await tester.pump();
    recorder.fail(StateError('stream ended unexpectedly'));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(transcribeCalls, 0);
    expect(find.textContaining('Microphone stream failed'), findsOneWidget);
  });

  testWidgets('workspace hanging stop is bounded and never calls STT', (
    tester,
  ) async {
    var transcribeCalls = 0;
    final speechController = _speechController(
      onTranscribe: () => transcribeCalls++,
    );
    final recorder = _FakeWorkspaceRecorder(hangOnStop: true);
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
      voiceStopTimeout: const Duration(milliseconds: 20),
    );

    final button = find.byKey(const ValueKey('hold-to-talk'));
    await tester.ensureVisible(button);
    final gesture = await tester.startGesture(tester.getCenter(button));
    await tester.pumpAndSettle();
    recorder.emit([1, 2, 3]);
    await tester.pump();
    await gesture.up();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 30));
    await tester.pump();

    expect(recorder.stopCalls, 1);
    expect(transcribeCalls, 0);
    expect(find.textContaining('Recording could not stop'), findsOneWidget);
  });

  testWidgets('workspace startup is invalidated when the screen unmounts', (
    tester,
  ) async {
    final speechController = _speechController(onTranscribe: () => 0);
    final recorder = _FakeWorkspaceRecorder(
      startDelay: const Duration(milliseconds: 100),
    );
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
    );

    final button = find.byKey(const ValueKey('hold-to-talk'));
    await tester.ensureVisible(button);
    await tester.tapAt(tester.getCenter(button));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();

    expect(recorder.disposed, isTrue);
  });

  testWidgets('workspace permission denial leaves text composer usable', (
    tester,
  ) async {
    final speechController = _speechController(onTranscribe: () => 0);
    final recorder = _FakeWorkspaceRecorder(permission: false);
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
    );

    final button = find.byKey(const ValueKey('hold-to-talk'));
    await tester.ensureVisible(button);
    final gesture = await tester.startGesture(tester.getCenter(button));
    await tester.pumpAndSettle();
    await gesture.cancel();

    expect(find.textContaining('Microphone permission denied'), findsOneWidget);
    expect(find.byKey(const ValueKey('assistant-composer')), findsOneWidget);
  });

  testWidgets('hold-to-talk remains accessible for keyboard and mouse users', (
    tester,
  ) async {
    final speechController = _speechController(onTranscribe: () => 0);
    final recorder = _FakeWorkspaceRecorder(permission: false);
    addTearDown(speechController.dispose);
    await _pumpVoiceScreen(
      tester,
      speechController: speechController,
      recorder: recorder,
    );

    expect(find.byType(HoldToTalkButton), findsOneWidget);
    expect(find.byTooltip('Send message'), findsOneWidget);
  });
}
