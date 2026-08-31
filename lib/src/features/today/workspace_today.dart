part of '../workspace/workspace.dart';

class TodayWorkspaceScreen extends StatefulWidget {
  const TodayWorkspaceScreen({
    super.key,
    required this.controller,
    required this.onSettings,
    this.speechController,
    this.voiceRecorderFactory,
    this.voiceStartupTimeout = const Duration(seconds: 10),
    this.voiceStopTimeout = const Duration(seconds: 10),
  });

  final WorkspaceController controller;
  final VoidCallback onSettings;
  final AppController? speechController;
  final WorkspaceAudioRecorderFactory? voiceRecorderFactory;
  final Duration voiceStartupTimeout;
  final Duration voiceStopTimeout;

  @override
  State<TodayWorkspaceScreen> createState() => _TodayWorkspaceScreenState();
}

class _TodayWorkspaceScreenState extends State<TodayWorkspaceScreen> {
  final _messageController = TextEditingController();

  @override
  void dispose() {
    _messageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: widget.controller,
      builder: (context, _) => WorkspacePageFrame(
        title: 'Today',
        subtitle: 'Keep work moving. Learn English in the context of the task.',
        compactActionInHeader: true,
        action: IconButton(
          onPressed: widget.onSettings,
          tooltip: 'Settings',
          icon: const Icon(Icons.settings_outlined),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            WorkspacePreviewNotice(origin: widget.controller.data.origin),
            if (widget.controller.data.capabilities.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.md),
              WorkspaceCapabilitySummary(
                capabilities: widget.controller.data.capabilities,
              ),
            ],
            if (widget.controller.loading) ...[
              const SizedBox(height: AppSpacing.md),
              const _WorkspaceLoadingBanner(),
            ],
            if (widget.controller.error != null) ...[
              const SizedBox(height: AppSpacing.md),
              _WorkspaceErrorBanner(
                message: widget.controller.error!,
                onRetry: widget.controller.load,
              ),
            ],
            const SizedBox(height: AppSpacing.xxl),
            _TodaySummary(
              data: widget.controller.data,
              cefr: widget.speechController?.cefr ?? 'B1',
            ),
            const SizedBox(height: AppSpacing.section),
            _TodayNextAction(
              data: widget.controller.data,
              onAsk: (prompt) => _submit(prompt),
            ),
            const SizedBox(height: AppSpacing.section),
            _TodayQuickCapture(controller: widget.controller),
            const SizedBox(height: AppSpacing.section),
            SectionTitle(
              'Ask your assistant',
              trailing: widget.speechController == null
                  ? 'Text-first'
                  : 'Text + voice',
            ),
            const SizedBox(height: AppSpacing.md),
            _AssistantChat(
              controller: widget.controller,
              messageController: _messageController,
              onSubmit: _submit,
              speechController: widget.speechController,
              recorderFactory: widget.voiceRecorderFactory,
              voiceStartupTimeout: widget.voiceStartupTimeout,
              voiceStopTimeout: widget.voiceStopTimeout,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _submit([String? value]) async {
    final text = (value ?? _messageController.text).trim();
    if (text.isEmpty || widget.controller.sending) return;
    _messageController.clear();
    await widget.controller.ask(text);
  }
}

class _TodayQuickCapture extends StatelessWidget {
  const _TodayQuickCapture({required this.controller});

  final WorkspaceController controller;

  @override
  Widget build(BuildContext context) {
    final canonical = controller.hasWorkspaceApi;
    return AppSurface(
      key: const ValueKey('today-quick-capture'),
      color: AppColors.surface,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SectionTitle(
            'Quick capture',
            trailing: canonical ? 'Writes to Work / Knowledge' : 'Preview only',
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(
            canonical
                ? 'Record the smallest useful piece of work while it is fresh.'
                : 'Connect the backend to save a task, decision or source.',
          ),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              OutlinedButton.icon(
                key: const ValueKey('today-quick-task'),
                onPressed: canonical
                    ? () => _showQuickCaptureTask(context, controller)
                    : null,
                icon: const Icon(Icons.checklist_outlined),
                label: const Text('New task'),
              ),
              OutlinedButton.icon(
                key: const ValueKey('today-quick-decision'),
                onPressed: canonical
                    ? () => _showCreateDecision(context, controller)
                    : null,
                icon: const Icon(Icons.gavel_outlined),
                label: const Text('Record decision'),
              ),
              OutlinedButton.icon(
                key: const ValueKey('today-quick-source'),
                onPressed: canonical
                    ? () => _showImportSource(context, controller)
                    : null,
                icon: const Icon(Icons.library_add_outlined),
                label: const Text('Add source'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _TodaySummary extends StatelessWidget {
  const _TodaySummary({required this.data, required this.cefr});

  final WorkspaceData data;
  final String cefr;

  @override
  Widget build(BuildContext context) {
    final overlay = LearningOverlayCopy.forLevel(cefr);
    final openTasks = data.tasks
        .where(
          (task) =>
              task.deletedAt == null &&
              task.status != 'Done' &&
              task.status != 'Cancelled',
        )
        .length;
    final metrics = [
      _SummaryCard(
        icon: Icons.task_alt,
        label: 'Focus',
        value: '$openTasks open task${openTasks == 1 ? '' : 's'}',
        color: AppColors.accent,
      ),
      _SummaryCard(
        icon: Icons.menu_book_outlined,
        label: 'Knowledge',
        value:
            '${data.sources.length} source${data.sources.length == 1 ? '' : 's'} connected',
        color: AppColors.success,
      ),
      _SummaryCard(
        icon: Icons.language,
        label: 'English layer',
        value: '${overlay.level} · ${overlay.label}',
        color: AppColors.warning,
      ),
    ];
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 620;
        return AppSurface(
          padding: EdgeInsets.zero,
          child: compact
              ? Column(
                  children: [
                    for (var i = 0; i < metrics.length; i++) ...[
                      Padding(
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        child: metrics[i],
                      ),
                      if (i < metrics.length - 1) const Divider(height: 1),
                    ],
                  ],
                )
              : Row(
                  children: [
                    for (var i = 0; i < metrics.length; i++) ...[
                      Expanded(
                        child: Padding(
                          padding: const EdgeInsets.all(AppSpacing.lg),
                          child: metrics[i],
                        ),
                      ),
                      if (i < metrics.length - 1)
                        const SizedBox(height: 48, child: VerticalDivider()),
                    ],
                  ],
                ),
        );
      },
    );
  }
}

class _TodayNextAction extends StatelessWidget {
  const _TodayNextAction({required this.data, required this.onAsk});

  final WorkspaceData data;
  final ValueChanged<String> onAsk;

  @override
  Widget build(BuildContext context) {
    final task = _nextTask(data.tasks);
    final decision = task == null ? _nextDecision(data.decisions) : null;
    if (task == null && decision == null) {
      return AppSurface(
        key: const ValueKey('today-next-action'),
        color: AppColors.surfaceMuted,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const AppIconBadge(
              icon: Icons.check_circle_outline,
              color: AppColors.success,
              backgroundColor: AppColors.successSoft,
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const EyebrowLabel('Next safe action'),
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    'No open work yet',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  const Text(
                    'Capture a task or decision and it will become the next practical step here.',
                  ),
                ],
              ),
            ),
          ],
        ),
      );
    }

    final title = task?.title ?? decision!.title;
    final detail = task != null
        ? '${task.priority} priority · ${task.status}${task.dueAt == null ? '' : ' · due ${_shortDate(task.dueAt!)}'}'
        : 'Decision to revisit · ${decision!.status}';
    final contextLine = task != null
        ? data.projectFor(task.projectId).name
        : 'Work decision';
    final prompt = task != null
        ? 'What is the next safe step for task "$title"?'
        : 'Help me revisit decision "$title" using the verified workspace context.';

    return Column(
      key: const ValueKey('today-next-action'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionTitle('Next safe action', trailing: 'From Work'),
        const SizedBox(height: AppSpacing.md),
        AppSurface(
          color: AppColors.surfaceAccent,
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const AppIconBadge(
                icon: Icons.north_east,
                color: AppColors.accent,
                backgroundColor: Colors.white,
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    EyebrowLabel(
                      task == null ? 'Decision' : 'Priority ${task.priority}',
                      color: AppColors.accentStrong,
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(title, style: Theme.of(context).textTheme.titleLarge),
                    const SizedBox(height: AppSpacing.xs),
                    Text(detail),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      contextLine,
                      style: Theme.of(context).textTheme.labelMedium,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    OutlinedButton.icon(
                      onPressed: () => onAsk(prompt),
                      icon: const Icon(Icons.auto_awesome, size: 18),
                      label: const Text('Ask for a safe next step'),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  WorkspaceTask? _nextTask(List<WorkspaceTask> tasks) {
    final open = tasks
        .where(
          (task) =>
              task.deletedAt == null &&
              task.status != 'Done' &&
              task.status != 'Cancelled',
        )
        .toList();
    open.sort((a, b) {
      final priority = _priorityWeight(
        b.priority,
      ).compareTo(_priorityWeight(a.priority));
      if (priority != 0) return priority;
      if (a.dueAt != null && b.dueAt != null) {
        final due = a.dueAt!.compareTo(b.dueAt!);
        if (due != 0) return due;
      } else if (a.dueAt != null) {
        return -1;
      } else if (b.dueAt != null) {
        return 1;
      }
      return a.title.toLowerCase().compareTo(b.title.toLowerCase());
    });
    return open.isEmpty ? null : open.first;
  }

  WorkspaceDecision? _nextDecision(List<WorkspaceDecision> decisions) {
    final open = decisions
        .where(
          (decision) =>
              decision.deletedAt == null &&
              decision.status != 'Rejected' &&
              decision.status != 'Superseded',
        )
        .toList();
    open.sort((a, b) => b.recordedAt.compareTo(a.recordedAt));
    return open.isEmpty ? null : open.first;
  }

  int _priorityWeight(String priority) => switch (priority.toLowerCase()) {
    'urgent' => 4,
    'high' => 3,
    'normal' => 2,
    'low' => 1,
    _ => 0,
  };

  String _shortDate(DateTime value) {
    final local = value.toLocal();
    return '${local.day}/${local.month}';
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({
    required this.icon,
    required this.label,
    required this.value,
    required this.color,
  });

  final IconData icon;
  final String label;
  final String value;
  final Color color;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      AppIconBadge(
        icon: icon,
        color: color,
        backgroundColor: color.withValues(alpha: 0.1),
        size: 36,
      ),
      const SizedBox(width: AppSpacing.md),
      Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            EyebrowLabel(label, color: color),
            const SizedBox(height: AppSpacing.xs),
            Text(value, style: Theme.of(context).textTheme.titleMedium),
          ],
        ),
      ),
    ],
  );
}
