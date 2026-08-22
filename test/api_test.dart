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
}
