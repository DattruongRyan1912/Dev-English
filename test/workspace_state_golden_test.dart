import 'dart:async';

import 'package:devenglish/src/api.dart';
import 'package:devenglish/src/screens/workspace_screen.dart';
import 'package:devenglish/src/theme.dart';
import 'package:devenglish/src/workspace_api.dart';
import 'package:devenglish/src/workspace_controller.dart';
import 'package:devenglish/src/workspace_models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _StateDelayedApi extends WorkspaceApi {
  _StateDelayedApi(this.bootstrapCompleter);

  final Completer<WorkspaceSnapshot> bootstrapCompleter;

  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) =>
      bootstrapCompleter.future;

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];
}

class _StateRetrievalApi extends WorkspaceApi {
  @override
  Future<List<WorkspaceSearchHit>> searchKnowledge(
    String query, {
    int limit = 20,
  }) async => const [
    WorkspaceSearchHit(
      id: 'state-hit',
      sourceId: 'source-release',
      sourceName: 'Release notes',
      revisionId: 'revision-stale',
      uri: 'manual://release-notes',
      excerpt:
          'This result is deliberately stale and uses the FTS fallback because the embedding sidecar is degraded.',
      freshness: 'Stale',
      origin: 'Canonical',
      retrievalMode: 'Degraded',
    ),
  ];
}

class _StateProviderFailureApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(
        data: WorkspaceData.empty(displayName: 'Ryan'),
        conversationId: '',
      );

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<WorkspaceAssistantResult> startConversation(String message) async {
    throw const ApiException(503, 'provider unavailable');
  }
}

class _StateOfflineApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async {
    throw TimeoutException('backend unavailable');
  }

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];
}

class _StateConflictApi extends WorkspaceApi {
  @override
  Future<WorkspaceProject> updateProject({
    required String projectId,
    String? name,
    String? description,
    String? status,
    required int expectedVersion,
    required String idempotencyKey,
  }) async {
    throw const ApiException(409, 'version conflict');
  }
}

class _StateSyncApi extends WorkspaceApi {
  @override
  Future<WorkspaceSnapshot> bootstrap({bool includeTrashed = false}) async =>
      const WorkspaceSnapshot(data: _emptyWorkspace, conversationId: '');

  @override
  Future<List<WorkspaceLearningObservation>> listLearningObservations({
    int limit = 20,
  }) async => const [];

  @override
  Future<WorkspaceSyncResult> syncDrive({
    String cursor = '',
    int pageSize = 100,
  }) async => const WorkspaceSyncResult(
    seen: 0,
    upserted: 0,
    skipped: 0,
    nextCursor: 'cursor-current',
    hasMore: false,
  );
}

const _emptyWorkspace = WorkspaceData.empty(displayName: 'Ryan');

void main() {
  testWidgets('loading state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    final bootstrap = Completer<WorkspaceSnapshot>();
    final controller = WorkspaceController(
      data: _emptyWorkspace,
      workspaceApi: _StateDelayedApi(bootstrap),
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: TodayWorkspaceScreen(controller: controller, onSettings: () {}),
        ),
      ),
    );
    final loading = controller.load();
    await tester.pump();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_loading.png'),
    );
    bootstrap.complete(
      const WorkspaceSnapshot(
        data: WorkspaceData.empty(displayName: 'Ryan'),
        conversationId: '',
      ),
    );
    await loading;
  });

  testWidgets('empty state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: const Scaffold(body: WorkWorkspaceScreen(data: _emptyWorkspace)),
      ),
    );
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_empty.png'),
    );
  });

  testWidgets('stale degraded retrieval state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    final controller = WorkspaceController(
      data: _emptyWorkspace,
      workspaceApi: _StateRetrievalApi(),
    );
    addTearDown(controller.dispose);
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: KnowledgeWorkspaceScreen(
            data: _emptyWorkspace,
            controller: controller,
          ),
        ),
      ),
    );
    await tester.enterText(
      find.byKey(const ValueKey('knowledge-search')),
      'release',
    );
    await tester.ensureVisible(find.byTooltip('Search knowledge'));
    await tester.tap(find.byTooltip('Search knowledge'));
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_stale-degraded.png'),
    );
  });

  testWidgets('provider error state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    final controller = WorkspaceController(
      data: _emptyWorkspace,
      workspaceApi: _StateProviderFailureApi(),
    );
    addTearDown(controller.dispose);
    await controller.load();
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: TodayWorkspaceScreen(controller: controller, onSettings: () {}),
        ),
      ),
    );
    await tester.enterText(
      find.byKey(const ValueKey('assistant-composer')),
      'What is in my workspace?',
    );
    await tester.ensureVisible(find.byTooltip('Send message'));
    await tester.tap(find.byTooltip('Send message'));
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_provider-error.png'),
    );
  });

  testWidgets('offline state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    final controller = WorkspaceController(
      data: _emptyWorkspace,
      workspaceApi: _StateOfflineApi(),
    );
    addTearDown(controller.dispose);
    await controller.load();
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: TodayWorkspaceScreen(controller: controller, onSettings: () {}),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_offline.png'),
    );
  });

  testWidgets('work conflict state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    const project = WorkspaceProject(
      id: 'project-1',
      name: 'Dev-English',
      summary: 'Workspace project',
      status: 'Active',
      version: 2,
    );
    const data = WorkspaceData(
      displayName: 'Ryan',
      origin: WorkspaceDataOrigin.canonical,
      projects: [project],
      tasks: [],
      decisions: [],
      sources: [],
      initialConversation: [],
    );
    final controller = WorkspaceController(
      data: data,
      workspaceApi: _StateConflictApi(),
    );
    addTearDown(controller.dispose);
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: AnimatedBuilder(
          animation: controller,
          builder: (context, _) => Scaffold(
            body: WorkWorkspaceScreen(data: data, controller: controller),
          ),
        ),
      ),
    );
    await controller.updateProject(project: project, name: 'Changed');
    await tester.pump();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_conflict.png'),
    );
  });

  testWidgets('connector sync state golden', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(390, 844));
    final controller = WorkspaceController(workspaceApi: _StateSyncApi());
    addTearDown(controller.dispose);
    await controller.load();
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        home: Scaffold(
          body: KnowledgeWorkspaceScreen(
            data: controller.data,
            controller: controller,
          ),
        ),
      ),
    );
    await tester.tap(find.text('Sync Drive'));
    await tester.pumpAndSettle();
    await expectLater(
      find.byType(Scaffold),
      matchesGoldenFile('goldens/workspace_state_390x844_sync.png'),
    );
  });
}
