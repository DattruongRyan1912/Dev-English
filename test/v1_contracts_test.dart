import 'dart:convert';

import 'package:devenglish/src/v1_contracts.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('V1 contracts round-trip work and knowledge data', () {
    final now = DateTime.utc(2026, 8, 26, 8, 30, 45);
    final evidence = EvidenceRef(
      id: 'evidence-1',
      sourceUri: 'docs/project/REQUIREMENTS.md',
      locator: '#timeout-policy',
      quote: 'Use a bounded timeout.',
    );
    final project = Project(
      id: 'project-1',
      name: 'DevEnglish',
      status: 'active',
      updatedAt: now,
    );
    final task = Task(
      id: 'task-1',
      projectId: project.id,
      title: 'Review timeout',
      status: 'todo',
      updatedAt: now,
    );
    final today = Today(
      date: '2026-08-26',
      focusTaskId: task.id,
      projects: <Project>[project],
      tasks: <Task>[task],
      decisions: <Decision>[],
    );
    final decoded = Today.fromJson(
      jsonDecode(jsonEncode(today.toJson())) as JsonObject,
    );

    expect(decoded.version, v1ContractVersion);
    expect(decoded.projects.single.name, project.name);
    expect(decoded.tasks.single.updatedAt, task.updatedAt);

    final response = AssistantResponse(
      answer: 'Use the documented timeout.',
      evidence: <EvidenceRef>[evidence],
      unknowns: <String>['Provider latency is not recorded.'],
      staleSources: <EvidenceRef>[],
    );
    final responseJson = response.toJson();
    expect(
      responseJson.keys,
      containsAll(<String>[
        'evidence',
        'unknowns',
        'staleSources',
        'suggestedActions',
        'actionReceipts',
      ]),
    );
    expect(
      AssistantResponse.fromJson(responseJson).unknowns,
      contains('Provider latency is not recorded.'),
    );
  });

  test('nullable values and action target fields remain JSON-safe', () {
    final challenge = ActionChallenge(
      id: 'challenge-1',
      action: 'github.issue.create',
      targetType: 'repository',
      targetId: 'repo-1',
      actionHash: 'hash-1',
      prompt: 'Create the issue?',
      expiresAt: DateTime.utc(2026, 8, 26, 9),
    );
    final receipt = ActionReceipt(
      id: 'receipt-1',
      challengeId: challenge.id,
      idempotencyKey: 'request-1',
      action: challenge.action,
      status: 'accepted',
      targetType: challenge.targetType,
      targetId: challenge.targetId,
    );

    final decodedChallenge = ActionChallenge.fromJson(
      jsonDecode(jsonEncode(challenge.toJson())) as JsonObject,
    );
    final decodedReceipt = ActionReceipt.fromJson(
      jsonDecode(jsonEncode(receipt.toJson())) as JsonObject,
    );

    expect(decodedChallenge.targetId, 'repo-1');
    expect(decodedChallenge.parameters, isEmpty);
    expect(decodedReceipt.challengeId, challenge.id);
    expect(decodedReceipt.message, isNull);
    expect(decodedReceipt.idempotencyKey, 'request-1');
    expect(AssistantResponse(answer: 'answer').toJson()['evidence'], isEmpty);
  });
}
