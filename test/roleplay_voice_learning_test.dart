import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/screens/roleplay_screen.dart';
import 'package:devenglish/src/theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart';
import 'package:http/testing.dart';
import 'package:record/record.dart';

const _scenarioJson = {
  'id': 'demo-incident',
  'title': 'Explain an incident to a teammate',
  'partnerRole': 'Senior engineer',
  'level': 'B1',
  'context': 'An API returns HTTP 500 for large uploads.',
  'goal': 'Explain the evidence, user impact and safe next step.',
  'type': 'incident',
};

class _FakeRoleplayRecorder implements RoleplayAudioRecorder {
  _FakeRoleplayRecorder({
    this.permission = true,
    this.startDelay = Duration.zero,
    this.tailBytes,
    this.hangOnStop = false,
  });

  final bool permission;
  final Duration startDelay;
  final List<int>? tailBytes;
  final bool hangOnStop;
  StreamController<Uint8List>? _chunks;
  Completer<void>? _stopGate;
  bool _tailEmitted = false;
  int startCalls = 0;
  int stopCalls = 0;
  bool disposed = false;

  bool get hasActiveListener => _chunks?.hasListener ?? false;

  @override
  Future<bool> hasPermission() async => permission;

  @override
  Future<Stream<Uint8List>> startStream(RecordConfig config) async {
    startCalls++;
    if (startDelay > Duration.zero) {
      await Future<void>.delayed(startDelay);
    }
    final chunks = StreamController<Uint8List>.broadcast();
    _chunks = chunks;
    _tailEmitted = false;
    if (disposed) await chunks.close();
    return chunks.stream;
  }

  @override
  Future<void> stop() async {
    stopCalls++;
    if (hangOnStop) {
      _stopGate ??= Completer<void>();
      await _stopGate!.future;
    }
    final chunks = _chunks;
    if (chunks != null && !chunks.isClosed) {
      if (tailBytes != null && !_tailEmitted) {
        _tailEmitted = true;
        chunks.add(Uint8List.fromList(tailBytes!));
      }
      await chunks.close();
    }
  }

  void completeStop() {
    if (_stopGate != null && !_stopGate!.isCompleted) {
      _stopGate!.complete();
    }
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

bool _containsBytes(List<int>? body, List<int> needle) {
  if (body == null || needle.isEmpty || needle.length > body.length) {
    return false;
  }
  for (var start = 0; start <= body.length - needle.length; start++) {
    var matches = true;
    for (var index = 0; index < needle.length; index++) {
      if (body[start + index] != needle[index]) {
        matches = false;
        break;
      }
    }
    if (matches) return true;
  }
  return false;
}

void main() {
  testWidgets('roleplay exposes editable voice transcript and learning help', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/roleplay/scenarios') {
        return Response(
          jsonEncode({
            'scenarios': [_scenarioJson],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/roleplay/conversations') {
        return Response(
          jsonEncode({
            'id': 'conversation-1',
            'roleplayType': 'demo-incident',
            'context': _scenarioJson['context'],
            'messages': [
              {
                'id': 'opening',
                'role': 'assistant',
                'content':
                    'Walk me through what happened, the user impact and your next step.',
              },
            ],
          }),
          200,
        );
      }
      return Response('{}', 404);
    });
    final controller = AppController(api: DevEnglishApi(client: client));
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(controller: controller),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();

    expect(find.text('Hold to talk'), findsOneWidget);
    expect(
      find.text('Voice transcript is editable before sending.'),
      findsOneWidget,
    );
    expect(find.text('Need a little help?'), findsOneWidget);
    expect(find.byTooltip('Read aloud'), findsOneWidget);
    expect(find.byKey(const ValueKey('roleplay-answer')), findsOneWidget);
  });

  test(
    'demo roleplay gives step-by-step help for Vietnamese guidance requests',
    () async {
      final client = MockClient((request) async {
        if (request.method == 'POST' &&
            request.url.path == '/api/v1/roleplay/conversations') {
          return Response(
            jsonEncode({
              'id': 'conversation-1',
              'roleplayType': 'demo-incident',
              'context': 'An API returns HTTP 500 for large uploads.',
              'messages': [
                {
                  'id': 'opening',
                  'role': 'assistant',
                  'content': 'Walk me through the incident.',
                },
              ],
            }),
            200,
          );
        }
        if (request.method == 'POST' &&
            request.url.path ==
                '/api/v1/roleplay/conversations/conversation-1/turns') {
          return Response('{}', 500);
        }
        return Response('{}', 404);
      });
      final controller = AppController(api: DevEnglishApi(client: client));
      addTearDown(controller.dispose);

      await controller.startRoleplay('demo-incident');
      await controller.sendRoleplayTurn(
        'Tôi chưa biết cách trả lời, bạn hướng dẫn tôi được không?',
      );

      expect(
        controller.conversation!.messages.last.content,
        contains('Let’s build the answer step by step'),
      );
      expect(
        controller.lastRoleplayTurn!.feedback.nextAction,
        contains('starter sentence'),
      );
      expect(controller.lastRoleplayTurn!.feedback.score, 0);
      expect(controller.lastRoleplayTurn!.feedback.scored, isFalse);
      expect(
        controller.lastRoleplayTurn!.feedback.summary,
        contains('not scored'),
      );
    },
  );

  test(
    'guidance fallback ignores helper words and normal technical answers',
    () async {
      final client = MockClient((request) async {
        if (request.method == 'POST' &&
            request.url.path == '/api/v1/roleplay/conversations') {
          return Response(
            jsonEncode({
              'id': 'conversation-1',
              'roleplayType': 'demo-incident',
              'context': 'An API returns HTTP 500 for large uploads.',
              'messages': [
                {
                  'id': 'opening',
                  'role': 'assistant',
                  'content': 'Walk me through the incident.',
                },
              ],
            }),
            200,
          );
        }
        if (request.method == 'POST' &&
            request.url.path ==
                '/api/v1/roleplay/conversations/conversation-1/turns') {
          return Response('{}', 500);
        }
        return Response('{}', 404);
      });
      final controller = AppController(api: DevEnglishApi(client: client));
      addTearDown(controller.dispose);

      await controller.startRoleplay('demo-incident');
      await controller.sendRoleplayTurn(
        'toi chua biet cach tra loi, huong dan toi voi',
      );
      expect(
        controller.conversation!.messages.last.content,
        contains('Let’s build the answer step by step'),
      );
      expect(
        controller.lastRoleplayTurn!.feedback.summary,
        contains('not scored'),
      );

      await controller.sendRoleplayTurn(
        'The helper service returns HTTP 500; the logs help explain the impact.',
      );
      expect(
        controller.conversation!.messages.last.content,
        isNot(contains('Let’s build the answer step by step')),
      );
      expect(
        controller.lastRoleplayTurn!.feedback.summary,
        'The explanation is understandable and work-focused.',
      );
    },
  );

  testWidgets('voice release transcribes while gesture cancel discards audio', (
    tester,
  ) async {
    var transcribeRequests = 0;
    List<int>? transcribedBody;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/roleplay/scenarios') {
        return Response(
          jsonEncode({
            'scenarios': [_scenarioJson],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/roleplay/conversations') {
        return Response(
          jsonEncode({
            'id': 'conversation-1',
            'roleplayType': 'demo-incident',
            'context': _scenarioJson['context'],
            'messages': [
              {
                'id': 'opening',
                'role': 'assistant',
                'content': 'Walk me through the incident.',
              },
            ],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/speaking/transcribe') {
        transcribeRequests++;
        transcribedBody = request.bodyBytes;
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
    });
    const tailBytes = [101, 102, 103, 104];
    final recorder = _FakeRoleplayRecorder(tailBytes: tailBytes);
    final controller = AppController(api: DevEnglishApi(client: client));
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(controller: controller, recorder: recorder),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();

    final button = find.byKey(const ValueKey('hold-to-talk'));
    final releaseGesture = await tester.startGesture(tester.getCenter(button));
    await tester.pump(const Duration(milliseconds: 50));
    recorder.emit([1, 2, 3, 4]);
    await tester.pump();
    await releaseGesture.up();
    await tester.pumpAndSettle();

    expect(transcribeRequests, 1);
    expect(_containsBytes(transcribedBody, tailBytes), isTrue);
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('roleplay-answer')))
          .controller
          ?.text,
      'I checked the logs.',
    );
    expect(find.textContaining('Transcript ready'), findsOneWidget);

    final cancelGesture = await tester.startGesture(tester.getCenter(button));
    await tester.pump(const Duration(milliseconds: 50));
    recorder.emit([5, 6, 7]);
    await tester.pump();
    await cancelGesture.cancel();
    await tester.pumpAndSettle();

    expect(transcribeRequests, 1);
    expect(find.textContaining('Recording cancelled'), findsOneWidget);
    expect(recorder.stopCalls, greaterThanOrEqualTo(2));
    expect(recorder.disposed, isFalse);

    final cappedGesture = await tester.startGesture(tester.getCenter(button));
    await tester.pump(const Duration(milliseconds: 50));
    recorder.emit(List<int>.filled(2 * 1024 * 1024 + 64, 8));
    await tester.pumpAndSettle();
    await cappedGesture.cancel();

    expect(transcribeRequests, 2);
  });

  testWidgets('voice startup is invalidated when the screen unmounts', (
    tester,
  ) async {
    var transcribeRequests = 0;
    final recorder = _FakeRoleplayRecorder(
      startDelay: const Duration(milliseconds: 100),
    );
    final controller = AppController(
      api: DevEnglishApi(
        client: MockClient((request) async {
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/speaking/transcribe') {
            transcribeRequests++;
          }
          return Response('{}', 404);
        }),
      ),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(controller: controller, recorder: recorder),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const ValueKey('hold-to-talk'))),
    );
    await tester.pump(const Duration(milliseconds: 1));
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(milliseconds: 150));
    await gesture.cancel();

    expect(recorder.startCalls, 1);
    expect(recorder.hasActiveListener, isFalse);
    expect(recorder.disposed, isTrue);
    expect(transcribeRequests, 0);
  });

  testWidgets('voice stream error is terminal before a release can call STT', (
    tester,
  ) async {
    var transcribeRequests = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/roleplay/scenarios') {
        return Response(
          jsonEncode({
            'scenarios': [_scenarioJson],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/roleplay/conversations') {
        return Response(
          jsonEncode({
            'id': 'conversation-1',
            'roleplayType': 'demo-incident',
            'context': _scenarioJson['context'],
            'messages': [
              {
                'id': 'opening',
                'role': 'assistant',
                'content': 'Walk me through the incident.',
              },
            ],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/speaking/transcribe') {
        transcribeRequests++;
        return Response(
          jsonEncode({
            'id': 'speech-1',
            'status': 'transcribed',
            'transcript': 'This must not be used.',
          }),
          200,
        );
      }
      return Response('{}', 404);
    });
    final recorder = _FakeRoleplayRecorder();
    final controller = AppController(api: DevEnglishApi(client: client));
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(controller: controller, recorder: recorder),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const ValueKey('hold-to-talk'))),
    );
    await tester.pump(const Duration(milliseconds: 50));
    recorder.emit([1, 2, 3]);
    await tester.pump();
    recorder.fail(StateError('stream ended unexpectedly'));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(transcribeRequests, 0);
    expect(find.textContaining('Microphone stream failed'), findsOneWidget);
  });

  testWidgets('hanging recorder stop stays serialized and never reaches STT', (
    tester,
  ) async {
    var transcribeRequests = 0;
    final recorder = _FakeRoleplayRecorder(hangOnStop: true);
    final controller = AppController(
      api: DevEnglishApi(
        client: MockClient((request) async {
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/speaking/transcribe') {
            transcribeRequests++;
          }
          return Response('{}', 404);
        }),
      ),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(
          controller: controller,
          recorder: recorder,
          recordingStopTimeout: const Duration(milliseconds: 20),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();

    final button = find.byKey(const ValueKey('hold-to-talk'));
    final gesture = await tester.startGesture(tester.getCenter(button));
    await tester.pumpAndSettle();
    recorder.emit([1, 2, 3, 4]);
    await tester.pump();
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 25));
    await tester.pump(const Duration(milliseconds: 25));
    await tester.pump();

    expect(recorder.stopCalls, 1);
    expect(transcribeRequests, 0);
    expect(find.textContaining('Recording could not stop'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump();
    expect(recorder.disposed, isTrue);
    expect(recorder.hasActiveListener, isFalse);

    recorder.completeStop();
    await tester.pump();
    expect(recorder.stopCalls, 1);
    expect(transcribeRequests, 0);
  });

  testWidgets('voice permission denial keeps the typed fallback available', (
    tester,
  ) async {
    final recorder = _FakeRoleplayRecorder(permission: false);
    final controller = AppController(
      api: DevEnglishApi(client: MockClient((_) async => Response('{}', 404))),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: RoleplayScreen(controller: controller, recorder: recorder),
      ),
    );
    await tester.pumpAndSettle();

    // Demo fallback provides the scenario when the test client has no backend.
    await tester.tap(find.text('Explain an incident to a teammate'));
    await tester.pumpAndSettle();
    await tester.startGesture(
      tester.getCenter(find.byKey(const ValueKey('hold-to-talk'))),
    );
    await tester.pumpAndSettle();

    expect(recorder.startCalls, 0);
    expect(find.textContaining('Permission denied'), findsOneWidget);
  });
}
