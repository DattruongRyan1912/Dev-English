part of '../workspace/workspace.dart';

class KnowledgeWorkspaceScreen extends StatefulWidget {
  const KnowledgeWorkspaceScreen({
    super.key,
    required this.data,
    this.controller,
    this.speechController,
  });

  final WorkspaceData data;
  final WorkspaceController? controller;
  final AppController? speechController;

  @override
  State<KnowledgeWorkspaceScreen> createState() =>
      _KnowledgeWorkspaceScreenState();
}

class _KnowledgeWorkspaceScreenState extends State<KnowledgeWorkspaceScreen> {
  final _searchController = TextEditingController();
  String _query = '';
  List<WorkspaceSearchHit> _remoteResults = const [];
  bool _searching = false;
  bool _syncing = false;
  WorkspaceSyncResult? _lastSync;
  String _lastSyncSource = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final sources = widget.data.sources.where(_matches).toList();
    final hasRemoteSearch =
        widget.controller != null &&
        !widget.data.origin.isPreview &&
        _query.isNotEmpty;
    return WorkspacePageFrame(
      title: 'Knowledge',
      subtitle: widget.data.origin.isPreview
          ? 'Search preview evidence before asking the assistant to act.'
          : 'Search source-backed facts before asking the assistant to act.',
      action: widget.controller == null
          ? null
          : Wrap(
              spacing: AppSpacing.sm,
              children: [
                OutlinedButton.icon(
                  onPressed: () => _showWorkspaceAssistant(
                    context,
                    widget.controller!,
                    speechController: widget.speechController,
                    initialPrompt: 'What is confirmed by these sources?',
                  ),
                  icon: const Icon(Icons.auto_awesome_outlined),
                  label: const Text('Ask assistant'),
                ),
                OutlinedButton.icon(
                  onPressed: _syncing ? null : _syncDrive,
                  icon: _syncing
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.sync),
                  label: const Text('Sync Drive'),
                ),
                OutlinedButton.icon(
                  onPressed: _syncing ? null : _syncGitHub,
                  icon: const Icon(Icons.code),
                  label: const Text('Sync GitHub'),
                ),
                FilledButton.icon(
                  onPressed: () =>
                      _showImportSource(context, widget.controller!),
                  icon: const Icon(Icons.add),
                  label: const Text('Add source'),
                ),
              ],
            ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WorkspacePreviewNotice(origin: widget.data.origin),
          if (widget.controller?.loading == true) ...[
            const SizedBox(height: AppSpacing.md),
            const _WorkspaceLoadingBanner(
              message: 'Refreshing sources and retrieval status...',
            ),
          ],
          if (widget.controller?.error != null) ...[
            const SizedBox(height: AppSpacing.md),
            _WorkspaceErrorBanner(
              message: widget.controller!.error!,
              onRetry: widget.controller!.load,
            ),
          ],
          if (_lastSync != null) ...[
            const SizedBox(height: AppSpacing.md),
            _KnowledgeSyncStatusCard(
              source: _lastSyncSource,
              result: _lastSync!,
            ),
          ],
          const SizedBox(height: AppSpacing.xxl),
          TextField(
            key: const ValueKey('knowledge-search'),
            controller: _searchController,
            onChanged: (value) => setState(() {
              _query = value.trim().toLowerCase();
              if (_query.isEmpty) _remoteResults = const [];
            }),
            onSubmitted: (_) => _searchRemote(),
            decoration: InputDecoration(
              prefixIcon: Icon(Icons.search),
              hintText: 'Search sources, claims or evidence...',
              suffixIcon: widget.controller == null
                  ? null
                  : IconButton(
                      tooltip: 'Search knowledge',
                      onPressed: _searching ? null : _searchRemote,
                      icon: _searching
                          ? const SizedBox(
                              width: 18,
                              height: 18,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.arrow_forward),
                    ),
            ),
          ),
          const SizedBox(height: AppSpacing.xxl),
          SectionTitle(
            'Connected sources',
            trailing: widget.data.origin.isPreview
                ? 'Preview only'
                : 'Canonical first',
          ),
          const SizedBox(height: AppSpacing.md),
          if (hasRemoteSearch && _searching)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.xl),
              child: Center(child: CircularProgressIndicator()),
            )
          else if (hasRemoteSearch && _remoteResults.isNotEmpty)
            ..._remoteResults.map(_KnowledgeSearchHitCard.new)
          else if (sources.isEmpty)
            const EmptyState(
              title: 'No matching source',
              description:
                  'The assistant will keep an unsupported detail unknown.',
            )
          else
            ...sources.map(
              (source) => _KnowledgeSourceCard(
                source: source,
                isPreview: widget.data.origin.isPreview,
                onOpen: widget.controller == null
                    ? null
                    : () => _openSourceDetail(source),
                onAsk:
                    widget.controller?.hasWorkspaceApi == true &&
                        !widget.data.origin.isPreview
                    ? () => _showWorkspaceAssistant(
                        context,
                        widget.controller!,
                        speechController: widget.speechController,
                        initialPrompt:
                            'What should I know or verify in "${source.title}"?',
                        assistantContext: WorkspaceAssistantContext(
                          entityType: 'source',
                          entityId: source.id,
                          label: source.title,
                        ),
                      )
                    : null,
              ),
            ),
          const SizedBox(height: AppSpacing.xxl),
          const _KnowledgeRulesCard(),
        ],
      ),
    );
  }

  bool _matches(KnowledgeSource source) {
    if (_query.isEmpty) return true;
    final haystack = [
      source.title,
      source.kind,
      ...source.claims.map((claim) => claim.statement),
      ...source.evidence.map((item) => item.excerpt),
    ].join(' ').toLowerCase();
    return haystack.contains(_query);
  }

  Future<void> _searchRemote() async {
    final controller = widget.controller;
    if (controller == null || _query.isEmpty || _searching) return;
    setState(() => _searching = true);
    final results = await controller.searchKnowledge(_query);
    if (!mounted) return;
    setState(() {
      _remoteResults = results;
      _searching = false;
    });
  }

  Future<void> _syncDrive() async {
    final controller = widget.controller;
    if (controller == null) return;
    setState(() => _syncing = true);
    final result = await controller.syncDrive();
    if (!mounted) return;
    setState(() {
      _syncing = false;
      if (result != null) {
        _lastSync = result;
        _lastSyncSource = 'Google Drive';
      }
    });
    if (result != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'Drive synced: ${result.upserted} new revision(s), ${result.skipped} unchanged.',
          ),
        ),
      );
    }
  }

  Future<void> _syncGitHub() async {
    final repository = await _askForRepository(context);
    if (repository == null || repository.trim().isEmpty) return;
    final controller = widget.controller;
    if (controller == null) return;
    setState(() => _syncing = true);
    final result = await controller.syncGitHub(repository);
    if (!mounted) return;
    setState(() {
      _syncing = false;
      if (result != null) {
        _lastSync = result;
        _lastSyncSource = 'GitHub';
      }
    });
    if (result != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'GitHub synced: ${result.upserted} new revision(s), ${result.skipped} unchanged.',
          ),
        ),
      );
    }
  }

  Future<void> _openSourceDetail(KnowledgeSource source) async {
    final controller = widget.controller;
    if (controller == null) return;
    final detail = await controller.knowledgeSourceDetail(source.id);
    if (!mounted || detail == null) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => _KnowledgeSourceDetailSheet(detail: detail),
    );
  }
}

class _KnowledgeSyncStatusCard extends StatelessWidget {
  const _KnowledgeSyncStatusCard({required this.source, required this.result});

  final String source;
  final WorkspaceSyncResult result;

  @override
  Widget build(BuildContext context) => AppSurface(
    key: const ValueKey('sync-status'),
    color: AppColors.surfaceAccent,
    borderColor: AppColors.success.withValues(alpha: 0.22),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Icon(Icons.sync, color: AppColors.success),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '$source sync complete',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    result.seen == 0
                        ? 'No changes found in the source.'
                        : '${result.seen} item(s) checked; ${result.upserted} new revision(s) imported and ${result.skipped} unchanged.',
                  ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: [
            StatusPill(
              label: result.hasMore ? 'More to sync' : 'Cursor up to date',
              color: result.hasMore ? AppColors.warning : AppColors.success,
              icon: result.hasMore ? Icons.more_horiz : Icons.check_circle,
            ),
            const StatusPill(
              label: 'Read-only import',
              color: AppColors.accent,
              icon: Icons.lock_outline,
            ),
          ],
        ),
      ],
    ),
  );
}

class _KnowledgeSearchHitCard extends StatelessWidget {
  const _KnowledgeSearchHitCard(this.hit);

  final WorkspaceSearchHit hit;

  @override
  Widget build(BuildContext context) => AppSurface(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    padding: const EdgeInsets.all(AppSpacing.lg),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const AppIconBadge(
              icon: Icons.manage_search_outlined,
              color: AppColors.success,
              backgroundColor: AppColors.successSoft,
              size: 36,
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(
                hit.sourceName,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            StatusChip(
              label: hit.freshness,
              color: _freshnessColor(hit.freshness),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        Text(hit.excerpt),
        const SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: [
            StatusChip(
              label: _retrievalLabel(hit.retrievalMode),
              color: _retrievalColor(hit.retrievalMode),
            ),
            if (hit.origin.isNotEmpty)
              StatusChip(
                label: hit.origin,
                color: hit.origin.toLowerCase() == 'canonical'
                    ? AppColors.success
                    : AppColors.accent,
              ),
          ],
        ),
        if (hit.uri.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sm),
          Text(hit.uri, style: Theme.of(context).textTheme.labelMedium),
        ],
      ],
    ),
  );
}

Color _freshnessColor(String value) {
  switch (value.toLowerCase()) {
    case 'stale':
      return AppColors.warning;
    case 'unknown':
      return AppColors.textSecondary;
    default:
      return AppColors.success;
  }
}

Color _retrievalColor(String value) {
  switch (value.toLowerCase()) {
    case 'degraded':
      return AppColors.warning;
    case 'lexical':
      return AppColors.accent;
    case 'context':
      return AppColors.accent;
    default:
      return AppColors.success;
  }
}

String _retrievalLabel(String value) {
  switch (value.toLowerCase()) {
    case 'degraded':
      return 'Degraded · FTS fallback';
    case 'lexical':
      return 'Exact text retrieval';
    case 'hybrid':
      return 'Hybrid retrieval';
    case 'context':
      return 'Pinned canonical context';
    default:
      return value.isEmpty ? 'Retrieval status unknown' : value;
  }
}

class _KnowledgeSourceCard extends StatelessWidget {
  const _KnowledgeSourceCard({
    required this.source,
    required this.isPreview,
    this.onOpen,
    this.onAsk,
  });

  final KnowledgeSource source;
  final bool isPreview;
  final VoidCallback? onOpen;
  final VoidCallback? onAsk;

  @override
  Widget build(BuildContext context) => AppSurface(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    padding: const EdgeInsets.all(AppSpacing.lg),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const AppIconBadge(icon: Icons.description_outlined, size: 38),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    source.title,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text('${source.kind} · ${source.updatedAt}'),
                ],
              ),
            ),
            StatusChip(
              label: isPreview ? 'Demo preview' : 'Canonical',
              color: isPreview ? AppColors.accent : AppColors.success,
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.lg),
        ...source.claims.map(
          (claim) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.md),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(
                  Icons.verified_outlined,
                  size: 18,
                  color: AppColors.success,
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(child: Text(claim.statement)),
                Text(
                  claim.confidence,
                  style: Theme.of(context).textTheme.labelMedium,
                ),
              ],
            ),
          ),
        ),
        const Divider(height: AppSpacing.xxl),
        const EyebrowLabel('Evidence', color: AppColors.textSecondary),
        const SizedBox(height: AppSpacing.sm),
        ...source.evidence.map(
          (evidence) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.sm),
            child: CitationCard(
              AssistantCitation(
                sourceId: source.id,
                sourceTitle: source.title,
                location: evidence.location,
                excerpt: evidence.excerpt,
                origin: isPreview
                    ? WorkspaceDataOrigin.demo
                    : WorkspaceDataOrigin.canonical,
              ),
            ),
          ),
        ),
        if (onOpen != null || onAsk != null) ...[
          const SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            children: [
              if (onOpen != null)
                TextButton.icon(
                  onPressed: onOpen,
                  icon: const Icon(Icons.timeline_outlined),
                  label: const Text('View revision timeline'),
                ),
              if (onAsk != null)
                TextButton.icon(
                  onPressed: onAsk,
                  icon: const Icon(Icons.auto_awesome_outlined),
                  label: const Text('Ask with this source'),
                ),
            ],
          ),
        ],
      ],
    ),
  );
}

class _KnowledgeSourceDetailSheet extends StatelessWidget {
  const _KnowledgeSourceDetailSheet({required this.detail});

  final KnowledgeSourceDetail detail;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: FractionallySizedBox(
        heightFactor: 0.9,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.sm,
            AppSpacing.xl,
            AppSpacing.xl,
          ),
          child: ListView(
            children: [
              Text(
                detail.source.title,
                style: Theme.of(context).textTheme.headlineSmall,
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(
                '${detail.source.kind} · ${detail.items.length} item(s) · ${detail.revisions.length} immutable revision(s)',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: AppSpacing.lg),
              const SectionTitle('Revision timeline'),
              const SizedBox(height: AppSpacing.sm),
              if (detail.revisions.isEmpty)
                const EmptyState(
                  title: 'No revisions yet',
                  description:
                      'This source has metadata but no ingested content.',
                )
              else
                ...detail.revisions.map(
                  (revision) => AppSurface(
                    margin: const EdgeInsets.only(bottom: AppSpacing.sm),
                    color:
                        detail.items.any(
                          (item) => item.currentRevisionId == revision.id,
                        )
                        ? AppColors.surfaceAccent
                        : AppColors.surface,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                revision.id,
                                style: Theme.of(context).textTheme.labelLarge,
                              ),
                            ),
                            if (detail.items.any(
                              (item) => item.currentRevisionId == revision.id,
                            ))
                              const StatusChip(
                                label: 'Current',
                                color: AppColors.success,
                              ),
                          ],
                        ),
                        const SizedBox(height: AppSpacing.xs),
                        Text(
                          revision.ingestedAt?.toIso8601String() ??
                              'Ingested recently',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          _truncate(revision.content),
                          maxLines: 4,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (detail.chunks.any(
                          (chunk) => chunk.revisionId == revision.id,
                        )) ...[
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            '${detail.chunks.where((chunk) => chunk.revisionId == revision.id).length} chunk(s) indexed',
                            style: Theme.of(context).textTheme.labelMedium,
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              const SizedBox(height: AppSpacing.lg),
              const SectionTitle('Evidence linked to this source'),
              const SizedBox(height: AppSpacing.sm),
              if (detail.evidence.isEmpty)
                const Text(
                  'No claim evidence is linked yet. The assistant must treat unsupported details as unknown.',
                )
              else
                ...detail.evidence.map(
                  (evidence) => CitationCard(
                    AssistantCitation(
                      sourceId: detail.source.id,
                      sourceTitle: detail.source.title,
                      location: evidence.location,
                      excerpt: evidence.excerpt,
                      origin: WorkspaceDataOrigin.canonical,
                    ),
                  ),
                ),
              const SizedBox(height: AppSpacing.md),
              Text(
                'Source data is canonical; summaries are only views over immutable revisions.',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _truncate(String value) {
    final normalized = value.trim();
    if (normalized.length <= 240) return normalized;
    return '${normalized.substring(0, 240)}…';
  }
}

class _KnowledgeRulesCard extends StatelessWidget {
  const _KnowledgeRulesCard();

  @override
  Widget build(BuildContext context) => AppSurface(
    color: AppColors.surfaceAccent,
    child: const _NoticeLine(
      icon: Icons.shield_outlined,
      color: AppColors.accent,
      text:
          'Canonical facts need evidence. If no evidence is available, the assistant must say unknown or inferred.',
    ),
  );
}
