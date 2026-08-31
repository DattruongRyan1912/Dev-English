part of '../workspace/workspace.dart';

class WorkWorkspaceScreen extends StatefulWidget {
  const WorkWorkspaceScreen({
    super.key,
    required this.data,
    this.controller,
    this.speechController,
  });

  final WorkspaceData data;
  final WorkspaceController? controller;
  final AppController? speechController;

  @override
  State<WorkWorkspaceScreen> createState() => _WorkWorkspaceScreenState();
}

class _WorkWorkspaceScreenState extends State<WorkWorkspaceScreen> {
  @override
  Widget build(BuildContext context) {
    final data = widget.data;
    final controller = widget.controller;
    return WorkspacePageFrame(
      title: 'Work',
      subtitle: data.origin.isPreview
          ? 'Preview of projects, tasks and decisions.'
          : 'Projects, tasks and decisions stay canonical here.',
      action: controller == null
          ? null
          : Wrap(
              spacing: AppSpacing.sm,
              children: [
                if (controller.hasWorkspaceApi)
                  OutlinedButton.icon(
                    onPressed: controller.actionBusy
                        ? null
                        : () => _showGitHubActionComposer(context, controller),
                    icon: const Icon(Icons.shield_outlined),
                    label: const Text('Safe GitHub action'),
                  ),
                OutlinedButton.icon(
                  onPressed: () => _showWorkspaceAssistant(
                    context,
                    controller,
                    speechController: widget.speechController,
                    initialPrompt: 'What should I do next in this workspace?',
                  ),
                  icon: const Icon(Icons.auto_awesome_outlined),
                  label: const Text('Ask assistant'),
                ),
                OutlinedButton.icon(
                  onPressed: controller.loading
                      ? null
                      : () => controller.load(
                          includeTrashed: !controller.includeTrashed,
                        ),
                  icon: Icon(
                    controller.includeTrashed
                        ? Icons.visibility_off_outlined
                        : Icons.delete_outline,
                  ),
                  label: Text(
                    controller.includeTrashed ? 'Hide trash' : 'Show trash',
                  ),
                ),
                OutlinedButton.icon(
                  onPressed: () => _showCreateDecision(context, controller),
                  icon: const Icon(Icons.gavel_outlined),
                  label: const Text('Decision'),
                ),
                FilledButton.icon(
                  onPressed: () => _showCreateProject(context, controller),
                  icon: const Icon(Icons.add),
                  label: const Text('New project'),
                ),
              ],
            ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WorkspacePreviewNotice(origin: data.origin),
          if (controller?.loading == true) ...[
            const SizedBox(height: AppSpacing.md),
            const _WorkspaceLoadingBanner(),
          ],
          if (controller?.error != null) ...[
            const SizedBox(height: AppSpacing.md),
            _WorkspaceErrorBanner(
              message: controller!.error!,
              onRetry: controller.load,
            ),
          ],
          if (controller?.lastActionReceipt case final receipt?) ...[
            const SizedBox(height: AppSpacing.md),
            _ActionReceiptCard(receipt: receipt),
          ],
          const SizedBox(height: AppSpacing.xxl),
          SectionTitle(
            'Projects',
            trailing: controller?.includeTrashed == true
                ? 'Active + trash'
                : 'Active',
          ),
          const SizedBox(height: AppSpacing.md),
          if (data.projects.isEmpty)
            const EmptyState(
              title: 'No projects yet',
              description:
                  'Create a project to give the assistant a work context.',
            )
          else
            ...data.projects.map(
              (project) => _ProjectCard(
                project: project,
                onAddTask: controller == null
                    ? null
                    : () => _showCreateTask(context, controller, project),
                onDetails: controller?.hasWorkspaceApi == true
                    ? () => _showProjectDetails(context, controller!, project)
                    : null,
                onAsk: controller?.hasWorkspaceApi == true
                    ? () => _showWorkspaceAssistant(
                        context,
                        controller!,
                        speechController: widget.speechController,
                        initialPrompt:
                            'What should I know or do next for "${project.name}"?',
                        assistantContext: WorkspaceAssistantContext(
                          entityType: 'project',
                          entityId: project.id,
                          label: project.name,
                        ),
                      )
                    : null,
                onEdit: controller == null
                    ? null
                    : () => _showEditProject(context, controller, project),
                onTrash: controller == null
                    ? null
                    : () => _confirmTrashProject(context, controller, project),
                onRestore: controller == null
                    ? null
                    : () => controller.restoreProject(project),
                onPurge: controller == null
                    ? null
                    : () => _confirmPurgeProject(context, controller, project),
              ),
            ),
          const SizedBox(height: AppSpacing.section),
          SectionTitle('Open tasks', trailing: data.origin.label),
          const SizedBox(height: AppSpacing.md),
          if (data.tasks.isEmpty)
            const EmptyState(
              title: 'No tasks yet',
              description:
                  'Tasks created here become available to the assistant.',
            )
          else
            ...data.tasks.map(
              (task) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: _TaskCard(
                  task: task,
                  project: data.projectFor(task.projectId),
                  onDetails: controller?.hasWorkspaceApi == true
                      ? () => _showTaskDetails(
                          context,
                          controller!,
                          task,
                          data.projectFor(task.projectId),
                        )
                      : null,
                  onAsk: controller?.hasWorkspaceApi == true
                      ? () => _showWorkspaceAssistant(
                          context,
                          controller!,
                          speechController: widget.speechController,
                          initialPrompt:
                              'What should I know or do next for "${task.title}"?',
                          assistantContext: WorkspaceAssistantContext(
                            entityType: 'task',
                            entityId: task.id,
                            label: task.title,
                          ),
                        )
                      : null,
                  onEdit: controller == null
                      ? null
                      : () => _showEditTask(context, controller, task),
                  onTrash: controller == null
                      ? null
                      : () => _confirmTrashTask(context, controller, task),
                  onRestore: controller == null
                      ? null
                      : () => controller.restoreTask(task),
                  onPurge: controller == null
                      ? null
                      : () => _confirmPurgeTask(context, controller, task),
                ),
              ),
            ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Recent decisions'),
          const SizedBox(height: AppSpacing.md),
          ...data.decisions.map(
            (decision) => _DecisionCard(
              decision: decision,
              onDetails: controller?.hasWorkspaceApi == true
                  ? () => _showDecisionDetails(context, controller!, decision)
                  : null,
              onAsk: controller?.hasWorkspaceApi == true
                  ? () => _showWorkspaceAssistant(
                      context,
                      controller!,
                      speechController: widget.speechController,
                      initialPrompt:
                          'What should I know or do next about "${decision.title}"?',
                      assistantContext: WorkspaceAssistantContext(
                        entityType: 'decision',
                        entityId: decision.id,
                        label: decision.title,
                      ),
                    )
                  : null,
              onEdit: controller == null
                  ? null
                  : () => _showEditDecision(context, controller, decision),
              onTrash: controller == null
                  ? null
                  : () => _confirmTrashDecision(context, controller, decision),
              onRestore: controller == null
                  ? null
                  : () => controller.restoreDecision(decision),
              onPurge: controller == null
                  ? null
                  : () => _confirmPurgeDecision(context, controller, decision),
            ),
          ),
        ],
      ),
    );
  }
}

class _ActionReceiptCard extends StatelessWidget {
  const _ActionReceiptCard({required this.receipt});

  final AssistantActionReceipt receipt;

  @override
  Widget build(BuildContext context) => AppSurface(
    key: const ValueKey('action-receipt'),
    color: AppColors.successSoft,
    borderColor: AppColors.success.withValues(alpha: 0.35),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const AppIconBadge(
          icon: Icons.verified_outlined,
          color: AppColors.success,
          backgroundColor: Colors.white,
        ),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const EyebrowLabel('Action receipt', color: AppColors.success),
              const SizedBox(height: AppSpacing.xs),
              Text(
                receipt.replayed
                    ? 'The same receipt was returned safely.'
                    : 'GitHub action confirmed.',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: AppSpacing.xs),
              Text('${receipt.operation} · ${receipt.status}'),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'Receipt ${receipt.id}',
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ],
          ),
        ),
      ],
    ),
  );
}

class _ProjectCard extends StatelessWidget {
  const _ProjectCard({
    required this.project,
    this.onDetails,
    this.onAsk,
    this.onAddTask,
    this.onEdit,
    this.onTrash,
    this.onRestore,
    this.onPurge,
  });

  final WorkspaceProject project;
  final VoidCallback? onDetails;
  final VoidCallback? onAsk;
  final VoidCallback? onAddTask;
  final VoidCallback? onEdit;
  final VoidCallback? onTrash;
  final VoidCallback? onRestore;
  final VoidCallback? onPurge;

  @override
  Widget build(BuildContext context) => AppSurface(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    padding: const EdgeInsets.all(AppSpacing.lg),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const AppIconBadge(icon: Icons.folder_open),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                project.name,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(project.summary),
            ],
          ),
        ),
        if (onAddTask != null)
          IconButton(
            tooltip: 'Add task',
            onPressed: onAddTask,
            icon: const Icon(Icons.add_task_outlined),
          ),
        if (onDetails != null ||
            onAsk != null ||
            onEdit != null ||
            onTrash != null ||
            onRestore != null)
          PopupMenuButton<String>(
            tooltip: 'Project actions',
            onSelected: (value) {
              if (value == 'details') onDetails?.call();
              if (value == 'ask') onAsk?.call();
              if (value == 'edit') onEdit?.call();
              if (value == 'trash') onTrash?.call();
              if (value == 'restore') onRestore?.call();
              if (value == 'purge') onPurge?.call();
            },
            itemBuilder: (context) => [
              if (onDetails != null)
                const PopupMenuItem(
                  value: 'details',
                  child: Text('Open details'),
                ),
              if (onAsk != null)
                const PopupMenuItem(
                  value: 'ask',
                  child: Text('Ask assistant with this context'),
                ),
              if (onEdit != null && project.deletedAt == null)
                const PopupMenuItem(value: 'edit', child: Text('Edit project')),
              if (onTrash != null && project.deletedAt == null)
                const PopupMenuItem(
                  value: 'trash',
                  child: Text('Move to trash'),
                ),
              if (onRestore != null && project.deletedAt != null)
                const PopupMenuItem(
                  value: 'restore',
                  child: Text('Restore project'),
                ),
              if (onPurge != null && project.deletedAt != null)
                const PopupMenuItem(
                  value: 'purge',
                  child: Text('Purge permanently'),
                ),
            ],
          ),
        const SizedBox(width: AppSpacing.md),
        StatusChip(
          label: project.deletedAt == null ? project.status : 'Trashed',
          color: project.deletedAt == null
              ? AppColors.success
              : AppColors.warning,
        ),
      ],
    ),
  );
}

class _TaskCard extends StatelessWidget {
  const _TaskCard({
    required this.task,
    required this.project,
    this.onDetails,
    this.onAsk,
    this.onEdit,
    this.onTrash,
    this.onRestore,
    this.onPurge,
  });

  final WorkspaceTask task;
  final WorkspaceProject project;
  final VoidCallback? onDetails;
  final VoidCallback? onAsk;
  final VoidCallback? onEdit;
  final VoidCallback? onTrash;
  final VoidCallback? onRestore;
  final VoidCallback? onPurge;

  @override
  Widget build(BuildContext context) => AppSurface(
    padding: const EdgeInsets.symmetric(
      horizontal: AppSpacing.lg,
      vertical: AppSpacing.md,
    ),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AppIconBadge(
          icon: task.priority == 'High' ? Icons.priority_high : Icons.task_alt,
          color: task.priority == 'High' ? AppColors.warning : AppColors.accent,
          backgroundColor: task.priority == 'High'
              ? AppColors.warningSoft
              : AppColors.accentSoft,
          size: 36,
        ),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(task.title, style: Theme.of(context).textTheme.bodyLarge),
              const SizedBox(height: AppSpacing.xs),
              Text('${project.name} · ${task.status}'),
            ],
          ),
        ),
        if (onDetails != null ||
            onAsk != null ||
            onEdit != null ||
            onTrash != null ||
            onRestore != null)
          PopupMenuButton<String>(
            tooltip: 'Task actions',
            onSelected: (value) {
              if (value == 'details') onDetails?.call();
              if (value == 'ask') onAsk?.call();
              if (value == 'edit') onEdit?.call();
              if (value == 'trash') onTrash?.call();
              if (value == 'restore') onRestore?.call();
              if (value == 'purge') onPurge?.call();
            },
            itemBuilder: (context) => [
              if (onDetails != null)
                const PopupMenuItem(
                  value: 'details',
                  child: Text('Open details'),
                ),
              if (onAsk != null)
                const PopupMenuItem(
                  value: 'ask',
                  child: Text('Ask assistant with this context'),
                ),
              if (onEdit != null && task.deletedAt == null)
                const PopupMenuItem(value: 'edit', child: Text('Edit task')),
              if (onTrash != null && task.deletedAt == null)
                const PopupMenuItem(
                  value: 'trash',
                  child: Text('Move to trash'),
                ),
              if (onRestore != null && task.deletedAt != null)
                const PopupMenuItem(
                  value: 'restore',
                  child: Text('Restore task'),
                ),
              if (onPurge != null && task.deletedAt != null)
                const PopupMenuItem(
                  value: 'purge',
                  child: Text('Purge permanently'),
                ),
            ],
          ),
        const SizedBox(width: AppSpacing.sm),
        StatusChip(
          label: task.deletedAt == null ? task.priority : 'Trashed',
          color: task.deletedAt == null ? AppColors.warning : AppColors.warning,
        ),
      ],
    ),
  );
}

class _DecisionCard extends StatelessWidget {
  const _DecisionCard({
    required this.decision,
    this.onDetails,
    this.onAsk,
    this.onEdit,
    this.onTrash,
    this.onRestore,
    this.onPurge,
  });

  final WorkspaceDecision decision;
  final VoidCallback? onDetails;
  final VoidCallback? onAsk;
  final VoidCallback? onEdit;
  final VoidCallback? onTrash;
  final VoidCallback? onRestore;
  final VoidCallback? onPurge;

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
              icon: Icons.gavel_outlined,
              color: AppColors.textSecondary,
              backgroundColor: AppColors.surfaceMuted,
              size: 34,
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(
                decision.title,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            if (onDetails != null ||
                onAsk != null ||
                onEdit != null ||
                onTrash != null ||
                onRestore != null)
              PopupMenuButton<String>(
                tooltip: 'Decision actions',
                onSelected: (value) {
                  if (value == 'details') onDetails?.call();
                  if (value == 'ask') onAsk?.call();
                  if (value == 'edit') onEdit?.call();
                  if (value == 'trash') onTrash?.call();
                  if (value == 'restore') onRestore?.call();
                  if (value == 'purge') onPurge?.call();
                },
                itemBuilder: (context) => [
                  if (onDetails != null)
                    const PopupMenuItem(
                      value: 'details',
                      child: Text('Open details'),
                    ),
                  if (onAsk != null)
                    const PopupMenuItem(
                      value: 'ask',
                      child: Text('Ask assistant with this context'),
                    ),
                  if (onEdit != null && decision.deletedAt == null)
                    const PopupMenuItem(
                      value: 'edit',
                      child: Text('Edit decision'),
                    ),
                  if (onTrash != null && decision.deletedAt == null)
                    const PopupMenuItem(
                      value: 'trash',
                      child: Text('Move to trash'),
                    ),
                  if (onRestore != null && decision.deletedAt != null)
                    const PopupMenuItem(
                      value: 'restore',
                      child: Text('Restore decision'),
                    ),
                  if (onPurge != null && decision.deletedAt != null)
                    const PopupMenuItem(
                      value: 'purge',
                      child: Text('Purge permanently'),
                    ),
                ],
              ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        Text(decision.outcome),
        const SizedBox(height: AppSpacing.sm),
        Text(
          decision.deletedAt == null
              ? decision.recordedAt
              : 'Trashed · ${decision.recordedAt}',
          style: Theme.of(context).textTheme.labelMedium,
        ),
      ],
    ),
  );
}

class StatusChip extends StatelessWidget {
  const StatusChip({super.key, required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) =>
      StatusPill(label: label, color: color, icon: Icons.circle);
}
