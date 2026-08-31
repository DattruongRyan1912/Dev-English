part of 'workspace.dart';

class WorkspacePageFrame extends StatelessWidget {
  const WorkspacePageFrame({
    super.key,
    required this.title,
    required this.subtitle,
    required this.child,
    this.action,
    this.compactActionInHeader = false,
  });

  final String title;
  final String subtitle;
  final Widget child;
  final Widget? action;
  final bool compactActionInHeader;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 600;
          final horizontal = compact ? AppSpacing.xl : AppSpacing.page;
          return SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              horizontal,
              compact ? AppSpacing.xl : AppSpacing.page,
              horizontal,
              AppSpacing.page,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 1120),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (compact && compactActionInHeader && action != null)
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Expanded(
                            child: _PageHeading(
                              title: title,
                              subtitle: subtitle,
                            ),
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          action!,
                        ],
                      )
                    else if (compact)
                      Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _PageHeading(title: title, subtitle: subtitle),
                          if (action != null) ...[
                            const SizedBox(height: AppSpacing.md),
                            Align(alignment: Alignment.topRight, child: action),
                          ],
                        ],
                      )
                    else
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Expanded(
                            child: _PageHeading(
                              title: title,
                              subtitle: subtitle,
                            ),
                          ),
                          if (action != null)
                            Flexible(
                              fit: FlexFit.loose,
                              child: Align(
                                alignment: Alignment.topRight,
                                child: action,
                              ),
                            ),
                        ],
                      ),
                    const SizedBox(height: AppSpacing.lg),
                    const Divider(),
                    const SizedBox(height: AppSpacing.page),
                    child,
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

class _PageHeading extends StatelessWidget {
  const _PageHeading({required this.title, required this.subtitle});

  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      EyebrowLabel('DevEnglish / $title'),
      const SizedBox(height: AppSpacing.sm),
      Text(title, style: Theme.of(context).textTheme.displaySmall),
      const SizedBox(height: AppSpacing.sm),
      ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 680),
        child: Text(subtitle, style: Theme.of(context).textTheme.bodyMedium),
      ),
    ],
  );
}

class WorkspacePreviewNotice extends StatelessWidget {
  const WorkspacePreviewNotice({
    super.key,
    this.origin = WorkspaceDataOrigin.demo,
  });

  final WorkspaceDataOrigin origin;

  @override
  Widget build(BuildContext context) {
    return AppSurface(
      color: AppColors.surfaceAccent,
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.md,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            origin.isPreview
                ? Icons.visibility_outlined
                : Icons.verified_outlined,
            color: origin.isPreview ? AppColors.accent : AppColors.success,
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  origin.isPreview
                      ? 'Preview workspace'
                      : 'Canonical workspace',
                  style: TextStyle(
                    color: AppColors.textPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                SizedBox(height: AppSpacing.xs),
                Text(
                  origin.isPreview
                      ? 'Read-only fixture data. Connect the backend to load your workspace.'
                      : 'Projects, tasks and source-backed context are loaded from the backend.',
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          StatusPill(
            label: origin.isPreview ? 'Preview' : 'Live',
            color: origin.isPreview ? AppColors.accent : AppColors.success,
            icon: origin.isPreview
                ? Icons.visibility_outlined
                : Icons.cloud_done_outlined,
          ),
        ],
      ),
    );
  }
}

class WorkspaceCapabilitySummary extends StatelessWidget {
  const WorkspaceCapabilitySummary({super.key, required this.capabilities});

  final Map<String, bool> capabilities;

  @override
  Widget build(BuildContext context) {
    const services = <(String, String)>[
      ('work', 'Work'),
      ('knowledge', 'Knowledge'),
      ('assistant', 'Assistant'),
      ('learning', 'Learning'),
    ];
    final enabled = services
        .where((service) => capabilities[service.$1] == true)
        .length;
    return AppSurface(
      key: const ValueKey('workspace-capability-summary'),
      color: AppColors.surfaceMuted,
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.md,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.tune_outlined, size: 18),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Workspace services',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              Text(
                '$enabled/${services.length} ready',
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              for (final service in services)
                StatusPill(
                  label:
                      '${service.$2} ${capabilities[service.$1] == true ? 'ready' : 'off'}',
                  color: capabilities[service.$1] == true
                      ? AppColors.success
                      : AppColors.textTertiary,
                  icon: capabilities[service.$1] == true
                      ? Icons.check_circle_outline
                      : Icons.remove_circle_outline,
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _WorkspaceLoadingBanner extends StatelessWidget {
  const _WorkspaceLoadingBanner({
    this.message = 'Refreshing canonical workspace data...',
  });

  final String message;

  @override
  Widget build(BuildContext context) => Semantics(
    container: true,
    liveRegion: true,
    label: message,
    child: AppSurface(
      key: const ValueKey('workspace-loading'),
      color: AppColors.surfaceAccent,
      borderColor: AppColors.accent.withValues(alpha: 0.2),
      padding: const EdgeInsets.all(AppSpacing.md),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          const SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(message, style: Theme.of(context).textTheme.bodyMedium),
          ),
        ],
      ),
    ),
  );
}

class _WorkspaceErrorBanner extends StatelessWidget {
  const _WorkspaceErrorBanner({required this.message, this.onRetry});

  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) => AppSurface(
    color: AppColors.errorSoft,
    borderColor: AppColors.error.withValues(alpha: 0.25),
    padding: const EdgeInsets.all(AppSpacing.md),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Icon(Icons.error_outline, color: AppColors.error),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                message,
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
              ),
              if (onRetry != null)
                Align(
                  alignment: Alignment.centerLeft,
                  child: TextButton.icon(
                    onPressed: onRetry,
                    icon: const Icon(Icons.refresh, size: 18),
                    label: const Text('Reload workspace'),
                  ),
                ),
            ],
          ),
        ),
      ],
    ),
  );
}
