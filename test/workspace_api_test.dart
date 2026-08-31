import 'dart:convert';

import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/workspace_api.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:devenglish/src/workspace_models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

class _WorkspaceClient extends http.BaseClient {
  final requests = <http.Request>[];

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final typedRequest = request as http.Request;
    requests.add(typedRequest);
    final path = typedRequest.url.path;
    final payload = switch (path) {
      '/api/v2/bootstrap' => {
        'workspace': {'id': 'workspace-test', 'name': 'Test workspace'},
        'projects': [
          {
            'id': 'project-1',
            'name': 'Release',
            'description': 'Verified release work',
            'status': 'active',
          },
        ],
        'tasks': [
          {
            'id': 'task-1',
            'projectId': 'project-1',
            'title': 'Check citations',
            'status': 'in_progress',
            'priority': 'high',
          },
        ],
        'decisions': <Map<String, dynamic>>[],
        'sources': [
          {
            'id': 'source-1',
            'title': 'Runbook',
            'kind': 'manual',
            'updatedAt': '2026-08-28T10:00:00Z',
          },
        ],
        'capabilities': {
          'platform': true,
          'work': true,
          'knowledge': true,
          'connectors': true,
          'assistant': true,
          'actions': true,
          'mcp': true,
          'learning': true,
        },
      },
      '/api/v2/projects' => {
        'id': 'project-2',
        'name': 'New project',
        'description': 'Description',
        'status': 'active',
      },
      '/api/v2/knowledge/sources' => {
        'source': {
          'id': 'source-2',
          'title': 'Imported notes',
          'kind': 'manual',
          'updatedAt': '2026-08-28T10:00:00Z',
        },
        'revisionId': 'revision-2',
        'chunkId': 'chunk-2',
      },
      '/api/v2/projects/project-1/history' => {
        'history': [
          {
            'id': 'history-1',
            'workspaceId': 'workspace-test',
            'actorUserId': 'user-test',
            'entityType': 'project',
            'entityId': 'project-1',
            'action': 'created',
            'fromVersion': 0,
            'toVersion': 1,
            'idempotencyKey': 'project-create-1',
            'createdAt': '2026-08-28T10:00:00Z',
          },
        ],
      },
      '/api/v2/assistant/conversations' => {
        'conversationId': 'conversation-1',
        'response': {
          'answer': 'The runbook says to verify the citation.',
          'grounding': 'grounded',
          'evidence': [
            {
              'evidenceId': 'knowledge-chunk-1',
              'quote': 'Verify the citation.',
              'locator': 'manual://runbook',
            },
          ],
          'unknowns': <String>[],
          'staleSources': <String>[],
          'suggestedActions': <Map<String, dynamic>>[],
          'actionReceipts': <Map<String, dynamic>>[],
        },
      },
      '/api/v2/knowledge/search' => {
        'results': [
          {
            'id': 'knowledge-chunk-1',
            'sourceId': 'source-1',
            'sourceName': 'Runbook',
            'revisionId': 'revision-1',
            'uri': 'manual://runbook',
            'excerpt': 'Verify the citation.',
            'freshness': 'current',
            'origin': 'canonical',
          },
        ],
      },
      '/api/v2/knowledge/sources/source-1/detail' => {
        'source': {
          'id': 'source-1',
          'title': 'Runbook',
          'kind': 'manual',
          'updatedAt': '2026-08-28T10:00:00Z',
        },
        'items': [
          {
            'id': 'item-1',
            'sourceId': 'source-1',
            'externalId': 'runbook',
            'title': 'Runbook',
            'currentRevisionId': 'revision-1',
          },
        ],
        'revisions': [
          {
            'id': 'revision-1',
            'sourceItemId': 'item-1',
            'contentHash': 'hash-1',
            'content': 'Verify the citation.',
            'ingestedAt': '2026-08-28T10:00:00Z',
          },
        ],
        'chunks': [
          {
            'id': 'knowledge-chunk-1',
            'revisionId': 'revision-1',
            'content': 'Verify the citation.',
            'ordinal': 0,
          },
        ],
        'evidence': [
          {
            'id': 'evidence-1',
            'sourceId': 'source-1',
            'location': 'manual://runbook',
            'excerpt': 'Verify the citation.',
          },
        ],
      },
      '/api/v2/actions/challenges' => {
        'action': {
          'actionId': 'action-1',
          'provider': 'github',
          'operation': 'github.issue.create',
          'challengeId': 'challenge-1',
          'idempotencyKey': 'action-key',
          'actionHash': 'hash-1',
          'targetType': 'github_repository',
          'targetId': 'owner/repo',
        },
        'target': {
          'operation': 'github.issue.create',
          'repository': 'owner/repo',
          'title': 'Track release',
          'body': 'Release checklist',
        },
        'expiresAt': '2026-08-28T10:05:00Z',
      },
      '/api/v2/actions/confirm' => {
        'action': {
          'actionId': 'action-1',
          'provider': 'github',
          'operation': 'github.issue.create',
          'challengeId': 'challenge-1',
          'idempotencyKey': 'action-key',
          'actionHash': 'hash-1',
          'targetType': 'github_repository',
          'targetId': 'owner/repo',
        },
        'receipt': {
          'id': 'receipt-1',
          'actionId': 'action-1',
          'provider': 'github',
          'operation': 'github.issue.create',
          'challengeId': 'challenge-1',
          'targetId': 'owner/repo',
          'status': 'accepted',
        },
        'replayed': false,
      },
      _ => <String, dynamic>{},
    };
    final status =
        path == '/api/v2/knowledge/search' ||
            path == '/api/v2/actions/confirm' ||
            path.endsWith('/history')
        ? 200
        : 201;
    return http.StreamedResponse(
      Stream<List<int>>.value(utf8.encode(jsonEncode(payload))),
      status,
      request: request,
      headers: const {'content-type': 'application/json'},
    );
  }
}

class _WorkTrashWorkspaceApi extends WorkspaceApi {
  WorkspaceActionTarget? previewedTarget;
  var confirmationCalls = 0;
  var bootstrapCalls = 0;

  @override
  Future<WorkspaceActionChallenge> createActionChallenge(
    WorkspaceActionTarget target, {
    String idempotencyKey = '',
  }) async {
    previewedTarget = target;
    return WorkspaceActionChallenge(
      action: WorkspaceActionBinding(
        actionId: 'action-trash',
        provider: 'work',
        operation: target.operation,
        challengeId: 'challenge-trash',
        idempotencyKey: 'trash-idempotency',
        actionHash: 'hash-trash',
        targetType: 'work_${target.entityType}',
        targetId: target.entityId,
      ),
      target: target,
      expiresAt: DateTime.utc(2026, 8, 29, 1, 5),
    );
  }

  @override
  Future<WorkspaceActionConfirmation> confirmAction(
    WorkspaceActionChallenge challenge, {
    String idempotencyKey = '',
  }) async {
    confirmationCalls++;
    return WorkspaceActionConfirmation(
      action: WorkspaceActionBinding(
        actionId: 'action-trash',
        provider: 'work',
        operation: challenge.action.operation,
        challengeId: 'challenge-trash',
        idempotencyKey: 'trash-idempotency',
        actionHash: 'hash-trash',
        targetType: 'work_${challenge.target.entityType}',
        targetId: challenge.target.entityId,
      ),
      receipt: AssistantActionReceipt(
        id: 'receipt-trash',
        actionId: 'action-trash',
        provider: 'work',
        operation: challenge.action.operation,
        challengeId: 'challenge-trash',
        targetId: challenge.target.entityId,
        status: 'accepted',
        replayed: false,
      ),
      replayed: false,
    );
  }

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async {
    bootstrapCalls++;
    return const WorkspaceSnapshot(
      data: WorkspaceData.empty(displayName: 'Ryan'),
      conversationId: '',
    );
  }

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];
}

void main() {
  test(
    'workspace API decodes the canonical bootstrap and search result',
    () async {
      final client = _WorkspaceClient();
      final workspaceApi = WorkspaceApi(
        api: DevEnglishApi(client: client, baseUrl: 'http://localhost:8080'),
      );

      final snapshot = await workspaceApi.bootstrap();
      expect(snapshot.data.origin.isPreview, isFalse);
      expect(snapshot.data.projects.single.name, 'Release');
      expect(snapshot.data.tasks.single.status, 'In Progress');
      expect(snapshot.data.tasks.single.priority, 'High');
      expect(snapshot.data.capabilities['work'], isTrue);
      expect(snapshot.data.capabilities['assistant'], isTrue);

      final results = await workspaceApi.searchKnowledge('citation');
      expect(results.single.sourceName, 'Runbook');
      expect(results.single.excerpt, 'Verify the citation.');

      final detail = await workspaceApi.knowledgeSourceDetail('source-1');
      expect(detail.source.title, 'Runbook');
      expect(detail.revisions.single.content, 'Verify the citation.');
      expect(detail.chunks.single.id, 'knowledge-chunk-1');
      expect(detail.evidence.single.location, 'manual://runbook');

      final history = await workspaceApi.projectHistory('project-1');
      expect(history.single.action, 'Created');
      expect(history.single.fromVersion, 0);
      expect(history.single.toVersion, 1);
      expect(history.single.actorUserId, 'user-test');
    },
  );

  test(
    'API preserves structured conflict status for the workspace UI',
    () async {
      final api = DevEnglishApi(
        client: _ConflictClient(),
        baseUrl: 'http://localhost:8080',
      );

      await expectLater(
        api.workspaceBootstrap(),
        throwsA(
          isA<ApiException>()
              .having((error) => error.statusCode, 'statusCode', 409)
              .having((error) => error.code, 'code', 'version_conflict'),
        ),
      );
    },
  );

  test(
    'workspace mutations send the idempotency header and assistant keeps citations',
    () async {
      final client = _WorkspaceClient();
      final workspaceApi = WorkspaceApi(
        api: DevEnglishApi(client: client, baseUrl: 'http://localhost:8080'),
      );

      await workspaceApi.createProject(
        name: 'New project',
        description: 'Description',
        idempotencyKey: 'project-test',
      );
      final imported = await workspaceApi.importManualSource(
        name: 'Imported notes',
        content: 'The source is reviewed before import.',
        idempotencyKey: 'source-test',
      );
      final assistant = await workspaceApi.startConversation(
        'What does the runbook say?',
      );

      expect(client.requests.first.headers['Idempotency-Key'], 'project-test');
      expect(client.requests[1].headers['Idempotency-Key'], 'source-test');
      expect(imported.source.title, 'Imported notes');
      expect(assistant.conversationId, 'conversation-1');
      expect(
        assistant.response.evidence.single.excerpt,
        'Verify the citation.',
      );
      expect(assistant.response.evidence.single.location, 'manual://runbook');

      await workspaceApi.startConversationWithContext(
        'What should I verify next?',
        context: const WorkspaceAssistantContext(
          entityType: 'task',
          entityId: 'task-1',
          label: 'Check citations',
        ),
      );
      expect(client.requests.last.body, contains('"contextType":"task"'));
      expect(client.requests.last.body, contains('"contextId":"task-1"'));
    },
  );

  test(
    'action preview and confirmation preserve the canonical payload',
    () async {
      final client = _WorkspaceClient();
      final workspaceApi = WorkspaceApi(
        api: DevEnglishApi(client: client, baseUrl: 'http://localhost:8080'),
      );
      const target = WorkspaceActionTarget(
        operation: 'github.issue.create',
        repository: 'owner/repo',
        title: 'Track release',
        body: 'Release checklist',
      );

      final challenge = await workspaceApi.createActionChallenge(
        target,
        idempotencyKey: 'challenge-key',
      );
      final confirmation = await workspaceApi.confirmAction(challenge);

      expect(challenge.action.challengeId, 'challenge-1');
      expect(challenge.target.repository, 'owner/repo');
      expect(confirmation.receipt.id, 'receipt-1');
      expect(confirmation.receipt.status, 'accepted');
      expect(
        client.requests[client.requests.length - 2].body,
        contains('owner/repo'),
      );
      expect(client.requests.last.body, contains('challenge-1'));
    },
  );

  test('work trash action target preserves version-bound fields', () {
    const target = WorkspaceActionTarget(
      operation: 'work.entity.trash',
      entityType: 'project',
      entityId: 'project-1',
      expectedVersion: 7,
    );

    expect(target.repository, isEmpty);
    expect(target.toJson(), {
      'operation': 'work.entity.trash',
      'repository': '',
      'entityType': 'project',
      'entityId': 'project-1',
      'expectedVersion': 7,
    });
    expect(WorkspaceActionTarget.fromJson(target.toJson()).expectedVersion, 7);
  });

  test(
    'controller routes work trash through challenge and receipt flow',
    () async {
      final api = _WorkTrashWorkspaceApi();
      final controller = WorkspaceController(workspaceApi: api);
      addTearDown(controller.dispose);

      await controller.trashProject(
        const WorkspaceProject(
          id: 'project-1',
          name: 'Release',
          summary: 'Canonical project',
          status: 'Active',
          version: 3,
        ),
      );

      expect(api.previewedTarget?.operation, 'work.entity.trash');
      expect(api.previewedTarget?.entityType, 'project');
      expect(api.previewedTarget?.entityId, 'project-1');
      expect(api.previewedTarget?.expectedVersion, 3);
      expect(api.confirmationCalls, 1);
      expect(api.bootstrapCalls, 1);
      expect(controller.lastActionReceipt?.provider, 'work');
      expect(controller.lastActionReceipt?.status, 'accepted');
    },
  );

  test(
    'controller routes permanent deletion through challenge and receipt flow',
    () async {
      final api = _WorkTrashWorkspaceApi();
      final controller = WorkspaceController(workspaceApi: api);
      addTearDown(controller.dispose);

      await controller.purgeProject(
        const WorkspaceProject(
          id: 'project-1',
          name: 'Release',
          summary: 'Trashed project',
          status: 'Trashed',
          version: 4,
        ),
      );

      expect(api.previewedTarget?.operation, 'work.entity.purge');
      expect(api.previewedTarget?.entityType, 'project');
      expect(api.previewedTarget?.entityId, 'project-1');
      expect(api.previewedTarget?.expectedVersion, 4);
      expect(api.confirmationCalls, 1);
      expect(api.bootstrapCalls, 1);
      expect(controller.lastActionReceipt?.operation, 'work.entity.purge');
    },
  );

  test('assistant response keeps typed action and receipt metadata', () {
    final response = AssistantResponse.fromJson({
      'answer': 'A change is ready to preview.',
      'grounding': 'grounded',
      'evidence': <Map<String, dynamic>>[],
      'unknowns': <String>[],
      'staleSources': <String>[],
      'suggestedActions': [
        {
          'id': 'action-1',
          'kind': 'github.issue.create',
          'label': 'Create release issue',
          'target': 'owner/repo',
          'requiresConfirmation': true,
        },
      ],
      'actionReceipts': [
        {
          'id': 'receipt-1',
          'operation': 'github.issue.create',
          'status': 'accepted',
          'replayed': true,
        },
      ],
    });

    expect(response.suggestedActions, ['Create release issue']);
    expect(response.actionDetails.single.requiresConfirmation, isTrue);
    expect(response.receiptDetails.single.replayed, isTrue);
  });

  test(
    'controller switches from demo to canonical data and uses persistent ask',
    () async {
      final client = _WorkspaceClient();
      final controller = WorkspaceController(
        workspaceApi: WorkspaceApi(
          api: DevEnglishApi(client: client, baseUrl: 'http://localhost:8080'),
        ),
      );
      addTearDown(controller.dispose);

      await controller.load();
      expect(controller.canonicalLoaded, isTrue);
      expect(controller.data.projects.single.name, 'Release');

      await controller.ask('What does the runbook say?');
      expect(controller.conversation.last.content, contains('runbook'));
      expect(
        controller.conversation.last.citations.single.excerpt,
        'Verify the citation.',
      );
    },
  );
}

class _ConflictClient extends http.BaseClient {
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async =>
      http.StreamedResponse(
        Stream<List<int>>.value(
          utf8.encode(
            jsonEncode({
              'error': {
                'code': 'version_conflict',
                'message': 'The record changed before this update.',
              },
            }),
          ),
        ),
        409,
        request: request,
        headers: const {'content-type': 'application/json'},
      );
}
