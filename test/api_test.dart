import 'dart:async';
import 'dart:convert';

import 'package:devenglish/src/api.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

class _RecordingClient extends http.BaseClient {
  http.Request? lastRequest;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    lastRequest = request as http.Request;
    final body = jsonEncode({
      'context': {'domain': 'database', 'sourceUrl': ''},
      'suggestedTerms': ['timeout'],
      'suggestedMission': {
        'id': 'mission-work-context',
        'title': 'Explain an API timeout',
        'mode': 'writing',
        'skillLabel': 'Technical Writing',
        'level': 'B1',
        'context': 'API timeout',
        'prompt': 'Explain the incident and next step.',
        'targetVocabulary': ['timeout'],
        'estimatedMinutes': 10,
      },
    });
    return http.StreamedResponse(
      Stream<List<int>>.value(utf8.encode(body)),
      201,
      request: request,
      headers: const {'content-type': 'application/json'},
    );
  }
}

class _HangingClient extends http.BaseClient {
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) =>
      Completer<http.StreamedResponse>().future;
}

class _SpeechClient extends http.BaseClient {
  http.BaseRequest? lastRequest;
  List<int>? lastBody;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    lastRequest = request;
    lastBody = await request.finalize().fold<List<int>>(
      <int>[],
      (body, chunk) => body..addAll(chunk),
    );
    return http.StreamedResponse(
      Stream<List<int>>.value(
        utf8.encode(
          jsonEncode({
            'id': 'speech-1',
            'status': 'transcribed',
            'transcript': 'hello',
          }),
        ),
      ),
      200,
      request: request,
      headers: const {'content-type': 'application/json'},
    );
  }
}

void main() {
  test('importWork posts the work context and decodes the mission', () async {
    final client = _RecordingClient();
    final api = DevEnglishApi(client: client, baseUrl: 'http://localhost:8080');

    final result = await api.importWork(
      sourceType: 'error',
      title: 'API timeout',
      content: 'The API returns HTTP 500 after a database timeout.',
    );

    expect(result.domain, 'database');
    expect(result.terms, ['timeout']);
    expect(result.mission.id, 'mission-work-context');
    expect(client.lastRequest?.url.path, '/api/v1/work-context');
    expect(
      jsonDecode(client.lastRequest!.body)['content'],
      'The API returns HTTP 500 after a database timeout.',
    );
  });

  test('importWork uses the explicit long-running timeout policy', () async {
    expect(
      DevEnglishApi.defaultWorkContextTimeout,
      const Duration(seconds: 90),
    );
    final api = DevEnglishApi(
      client: _HangingClient(),
      baseUrl: 'http://localhost:8080',
      workContextTimeout: const Duration(milliseconds: 25),
    );

    await expectLater(
      api.importWork(
        sourceType: 'error',
        title: 'API timeout',
        content: 'The API returns HTTP 500 after a database timeout.',
      ),
      throwsA(isA<TimeoutException>()),
    );
  });

  test(
    'transcribeAudio sends a WAV multipart with matching MIME and filename',
    () async {
      final client = _SpeechClient();
      final api = DevEnglishApi(
        client: client,
        baseUrl: 'http://localhost:8080',
      );

      final result = await api.transcribeAudio(
        bytes: [1, 2, 3, 4],
        mimeType: 'audio/wav',
        missionId: 'mission-1',
      );

      expect(result.transcript, 'hello');
      expect(
        client.lastRequest!.headers['content-type'],
        startsWith('multipart/form-data; boundary='),
      );
      final body = utf8.decode(client.lastBody!);
      expect(body, contains('content-disposition: form-data; name="audio"'));
      expect(body, contains('filename="recording.wav"'));
      expect(body, contains('content-type: audio/wav'));
      expect(body, contains('name="missionId"'));
      expect(body, contains('mission-1'));
    },
  );

  test('transcribeAudio uses the bounded speech timeout', () async {
    final api = DevEnglishApi(
      client: _HangingClient(),
      baseUrl: 'http://localhost:8080',
      speechTimeout: const Duration(milliseconds: 25),
    );

    await expectLater(
      api.transcribeAudio(bytes: [1, 2, 3], mimeType: 'audio/wav'),
      throwsA(isA<TimeoutException>()),
    );
  });
}
