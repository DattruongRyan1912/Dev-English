import 'dart:async';
import 'dart:convert';

import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/app_controller.dart';
import 'package:devenglish/src/screens/roleplay_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

class _DelayedTurnClient extends http.BaseClient {
  _DelayedTurnClient(this.turn);

  final Completer<void> turn;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final path = request.url.path;
    late final Object body;
    if (path.endsWith('/roleplay/scenarios')) {
      body = {
        'scenarios': [
          {
            'id': 'system-design',
            'title': 'System design',
            'partnerRole': 'Staff engineer',
            'level': 'B2',
            'goal': 'Explain architecture clearly.',
            'context': 'A retry-safe API',
            'opening': 'How would you design this system?',
            'type': 'system-design',
          },
        ],
      };
    } else if (path.endsWith('/roleplay/conversations')) {
      body = {
        'id': 'conversation-1',
        'roleplayType': 'system-design',
        'context': 'A retry-safe API',
        'messages': [
          {
            'id': 'm1',
            'role': 'assistant',
            'content': 'How would you design this system?',
          },
        ],
      };
    } else if (path.contains('/turns')) {
      await turn.future;
      body = {
        'conversation': {
          'id': 'conversation-1',
          'roleplayType': 'system-design',
          'context': 'A retry-safe API',
          'messages': [
            {
              'id': 'm1',
              'role': 'assistant',
              'content': 'How would you design this system?',
            },
            {
              'id': 'm2',
              'role': 'user',
              'content': 'I would start with timeouts and retries.',
            },
            {
              'id': 'm3',
              'role': 'assistant',
              'content': 'How will you validate that retry policy?',
            },
          ],
        },
        'feedback': {
          'score': 72,
          'summary': 'Clear starting point.',
          'whatWasGood': ['You named a concrete control.'],
          'mainIssue': 'Add evidence and user impact.',
          'nextAction': 'Explain how you will validate the retry.',
          'corrections': [],
        },
      };
    } else {
      throw StateError('unexpected path $path');
    }
    return http.StreamedResponse(
      Stream<List<int>>.value(utf8.encode(jsonEncode(body))),
      200,
      request: request,
      headers: const {'content-type': 'application/json'},
    );
  }
}

void main() {
  testWidgets('roleplay shows the user message before the partner replies', (
    tester,
  ) async {
    final turn = Completer<void>();
    final controller = AppController(
      api: DevEnglishApi(
        client: _DelayedTurnClient(turn),
        baseUrl: 'http://localhost:8080',
      ),
    );
    addTearDown(controller.dispose);

    await controller.startRoleplay('system-design');
    await tester.pumpWidget(
      MaterialApp(home: RoleplayScreen(controller: controller)),
    );
    await tester.pump();

    expect(find.text('How would you design this system?'), findsOneWidget);

    await tester.enterText(
      find.byType(TextField),
      'I would start with timeouts and retries.',
    );
    await tester.tap(find.byTooltip('Send reply'));
    await tester.pump();

    expect(
      find.text('I would start with timeouts and retries.'),
      findsOneWidget,
    );
    expect(find.text('Partner is thinking...'), findsOneWidget);
    expect(find.text('How will you validate that retry policy?'), findsNothing);

    turn.complete();
    await tester.pumpAndSettle();

    expect(
      find.text('I would start with timeouts and retries.'),
      findsOneWidget,
    );
    expect(find.text('Partner is thinking...'), findsNothing);
    expect(
      find.text('How will you validate that retry policy?'),
      findsOneWidget,
    );
  });
}
