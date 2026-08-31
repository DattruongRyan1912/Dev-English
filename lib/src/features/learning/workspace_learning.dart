part of '../workspace/workspace.dart';

class LearningWorkspaceScreen extends StatefulWidget {
  const LearningWorkspaceScreen({
    super.key,
    required this.controller,
    this.workspaceController,
    required this.onPractice,
    required this.onReview,
    required this.onProgress,
    required this.onDiagnostic,
    required this.onRoleplay,
    required this.onCopilot,
    required this.onSpeaking,
  });

  final AppController controller;
  final WorkspaceController? workspaceController;
  final VoidCallback onPractice;
  final VoidCallback onReview;
  final VoidCallback onProgress;
  final VoidCallback onDiagnostic;
  final VoidCallback onRoleplay;
  final VoidCallback onCopilot;
  final VoidCallback onSpeaking;

  @override
  State<LearningWorkspaceScreen> createState() =>
      _LearningWorkspaceScreenState();
}

class _LearningWorkspaceScreenState extends State<LearningWorkspaceScreen> {
  late final TextEditingController _observationController;
  bool _savingObservation = false;
  bool _observationSaved = false;

  @override
  void initState() {
    super.initState();
    _observationController = TextEditingController();
  }

  @override
  void dispose() {
    _observationController.dispose();
    super.dispose();
  }

  WorkspaceTask? get _nextTask {
    final controller = widget.workspaceController;
    if (controller == null) return null;
    for (final task in controller.data.tasks) {
      if (task.deletedAt == null &&
          task.status != 'Done' &&
          task.status != 'Cancelled') {
        return task;
      }
    }
    return null;
  }

  WorkspaceDecision? get _nextDecision {
    final controller = widget.workspaceController;
    if (controller == null) return null;
    for (final decision in controller.data.decisions) {
      if (decision.deletedAt == null && decision.status != 'Rejected') {
        return decision;
      }
    }
    return null;
  }

  Future<void> _saveObservation() async {
    final workspaceController = widget.workspaceController;
    final response = _observationController.text.trim();
    if (workspaceController == null || response.isEmpty || _savingObservation) {
      return;
    }
    final task = _nextTask;
    final decision = _nextDecision;
    final sourceType = task == null
        ? (decision == null ? 'learning' : 'decision')
        : 'task';
    final sourceId = task?.id ?? decision?.id ?? '';
    final prompt = task == null
        ? (decision == null
              ? 'Explain one useful next step from your current workflow.'
              : 'Explain the decision and its trade-off in simple English.')
        : 'Explain the next safe step for this task in simple English.';
    setState(() {
      _savingObservation = true;
      _observationSaved = false;
    });
    final saved = await workspaceController.recordLearningObservation(
      sourceType: sourceType,
      sourceId: sourceId,
      skill: 'technical_writing',
      prompt: prompt,
      response: response,
    );
    if (!mounted) return;
    setState(() {
      _savingObservation = false;
      _observationSaved = saved;
    });
    if (saved) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Learning note saved separately from Work.'),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final legacyReady = widget.controller.legacyLoaded;
    final home = widget.controller.home;
    final task = _nextTask;
    final decision = _nextDecision;
    final workspace = widget.workspaceController;
    final observations = workspace?.learningObservations ?? const [];
    final overlay = LearningOverlayCopy.forLevel(widget.controller.cefr);
    return WorkspacePageFrame(
      title: 'Learning',
      subtitle: 'A quiet English layer around the work you already do.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WorkspacePreviewNotice(
            origin: workspace?.data.origin ?? WorkspaceDataOrigin.demo,
          ),
          const SizedBox(height: AppSpacing.xxl),
          AppSurface(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Row(
              children: [
                SizedBox(
                  width: 82,
                  height: 82,
                  child: Stack(
                    alignment: Alignment.center,
                    children: [
                      CircularProgressIndicator(
                        value: legacyReady ? home.state.overallScore / 100 : 0,
                        strokeWidth: 7,
                        backgroundColor: AppColors.border,
                      ),
                      Text(
                        legacyReady
                            ? '${home.state.overallScore.round()}'
                            : '—',
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: AppSpacing.xl),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const EyebrowLabel('Current layer'),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        legacyReady
                            ? '${overlay.level} · ${home.state.weakestSkill.replaceAll('_', ' ')}'
                            : 'Workflow overlay ready',
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      const Text(
                        'Learning observations stay separate from canonical work data.',
                      ),
                      if (!legacyReady) ...[
                        const SizedBox(height: AppSpacing.xs),
                        const Text(
                          'Legacy learning metrics are unavailable; work-derived learning remains available.',
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          _WorkflowMissionCard(
            task: task,
            decision: decision,
            responseController: _observationController,
            overlay: overlay,
            saving: _savingObservation,
            saved: _observationSaved,
            enabled: widget.workspaceController != null,
            onSave: _saveObservation,
          ),
          if (observations.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.section),
            const SectionTitle(
              'Recent learning notes',
              trailing: 'Separate from Work',
            ),
            const SizedBox(height: AppSpacing.md),
            ...observations.map(_LearningObservationCard.new),
          ],
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Choose a focused practice'),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              _LearningAction(
                icon: Icons.edit_note_rounded,
                label: 'Practice',
                onPressed: widget.onPractice,
              ),
              _LearningAction(
                icon: Icons.replay_outlined,
                label: 'Review',
                onPressed: widget.onReview,
              ),
              _LearningAction(
                icon: Icons.forum_outlined,
                label: 'Roleplay',
                onPressed: widget.onRoleplay,
              ),
              _LearningAction(
                icon: Icons.mic_none_outlined,
                label: 'Speaking',
                onPressed: widget.onSpeaking,
              ),
              _LearningAction(
                icon: Icons.translate_outlined,
                label: 'English Copilot',
                onPressed: widget.onCopilot,
              ),
              _LearningAction(
                icon: Icons.insights_outlined,
                label: 'Progress',
                onPressed: widget.onProgress,
              ),
              _LearningAction(
                icon: Icons.fact_check_outlined,
                label: 'Diagnostic',
                onPressed: widget.onDiagnostic,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xxl),
          LearningSupportCard(
            title: 'Need a little help? · ${overlay.level}',
            explanation: overlay.explanation,
            starter: overlay.supportStarter,
            followUp: overlay.followUp,
          ),
        ],
      ),
    );
  }
}

class _LearningObservationCard extends StatelessWidget {
  const _LearningObservationCard(this.observation);

  final WorkspaceLearningObservation observation;

  @override
  Widget build(BuildContext context) => AppSurface(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const AppIconBadge(
              icon: Icons.bookmark_outline,
              color: AppColors.warning,
              backgroundColor: AppColors.warningSoft,
              size: 34,
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(
                observation.skill.replaceAll('_', ' '),
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            if (observation.createdAt != null)
              Text(
                _shortDate(observation.createdAt!),
                style: Theme.of(context).textTheme.labelMedium,
              ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        Text(observation.response),
        if (observation.sourceType.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sm),
          Text(
            'From ${observation.sourceType}${observation.sourceId.isEmpty ? '' : ' · ${observation.sourceId}'}',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ],
    ),
  );
}

class _WorkflowMissionCard extends StatelessWidget {
  const _WorkflowMissionCard({
    required this.task,
    required this.decision,
    required this.responseController,
    required this.overlay,
    required this.saving,
    required this.saved,
    required this.enabled,
    required this.onSave,
  });

  final WorkspaceTask? task;
  final WorkspaceDecision? decision;
  final TextEditingController responseController;
  final LearningOverlayCopy overlay;
  final bool saving;
  final bool saved;
  final bool enabled;
  final VoidCallback onSave;

  @override
  Widget build(BuildContext context) {
    final workTitle = task?.title ?? decision?.title;
    final sourceLabel = task == null
        ? (decision == null ? 'Workflow prompt' : 'From Work · decision')
        : 'From Work · task';
    final prompt = task == null
        ? (decision == null
              ? 'What is one useful next step in your current workflow?'
              : 'How would you explain this decision and its trade-off?')
        : 'How would you explain the next safe step for this task?';
    return AppSurface(
      color: AppColors.surfaceAccent,
      padding: const EdgeInsets.all(AppSpacing.xl),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const AppIconBadge(icon: Icons.auto_awesome_outlined),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const EyebrowLabel('Learn from the work in front of you'),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      sourceLabel,
                      style: Theme.of(context).textTheme.labelMedium,
                    ),
                  ],
                ),
              ),
              if (saved)
                const Icon(Icons.check_circle, color: AppColors.success),
            ],
          ),
          const SizedBox(height: AppSpacing.lg),
          Text(
            workTitle ?? prompt,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(
            workTitle == null
                ? 'Trả lời bằng tiếng Việt hoặc tiếng Anh đều được. Nội dung chỉ là learning observation.'
                : 'Không cần dịch toàn bộ. Hãy nói một câu tiếng Anh ngắn về việc bạn sẽ làm tiếp theo.',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
          const SizedBox(height: AppSpacing.md),
          AppSurface(
            color: AppColors.surface,
            padding: const EdgeInsets.all(AppSpacing.md),
            child: Text(
              task == null && decision == null
                  ? 'Starter: ${overlay.workflowStarter}'
                  : 'Starter: ${overlay.taskStarter}',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          Material(
            type: MaterialType.transparency,
            child: TextField(
              key: const ValueKey('learning-observation-response'),
              controller: responseController,
              enabled: enabled && !saving,
              minLines: 2,
              maxLines: 5,
              textInputAction: TextInputAction.newline,
              decoration: const InputDecoration(
                labelText: 'Your answer',
                hintText:
                    'I will inspect the logs before changing the retry policy.',
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          LayoutBuilder(
            builder: (context, constraints) {
              final note = Text(
                enabled
                    ? 'Learning note không thay đổi task, project hay decision.'
                    : 'Connect the canonical workspace to save this note.',
                style: Theme.of(context).textTheme.bodySmall,
              );
              final saveButton = FilledButton.icon(
                style: FilledButton.styleFrom(minimumSize: const Size(0, 48)),
                onPressed: enabled && !saving ? onSave : null,
                icon: saving
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.bookmark_add_outlined),
                label: Text(saving ? 'Saving' : 'Save note'),
              );
              final narrow =
                  !constraints.hasBoundedWidth || constraints.maxWidth < 520;
              if (narrow) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    note,
                    const SizedBox(height: AppSpacing.md),
                    saveButton,
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: note),
                  const SizedBox(width: AppSpacing.md),
                  saveButton,
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}

class _LearningAction extends StatelessWidget {
  const _LearningAction({
    required this.icon,
    required this.label,
    required this.onPressed,
  });

  final IconData icon;
  final String label;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onPressed,
          borderRadius: BorderRadius.circular(AppRadius.small),
          child: Container(
            constraints: const BoxConstraints(minHeight: 52),
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.md,
              vertical: AppSpacing.sm,
            ),
            decoration: BoxDecoration(
              color: AppColors.surface,
              borderRadius: BorderRadius.circular(AppRadius.small),
              border: Border.all(color: AppColors.borderStrong),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, color: AppColors.accent, size: 20),
                const SizedBox(width: AppSpacing.sm),
                Text(label, style: Theme.of(context).textTheme.labelLarge),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
