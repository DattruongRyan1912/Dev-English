part of 'workspace.dart';

class _ProjectSelector extends StatelessWidget {
  const _ProjectSelector({
    required this.projects,
    required this.value,
    required this.onChanged,
  });

  final List<WorkspaceProject> projects;
  final String value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) => DropdownButtonFormField<String>(
    initialValue: projects.any((project) => project.id == value) ? value : '',
    decoration: const InputDecoration(labelText: 'Project (optional)'),
    items: [
      const DropdownMenuItem(value: '', child: Text('No project')),
      ...projects.map(
        (project) =>
            DropdownMenuItem(value: project.id, child: Text(project.name)),
      ),
    ],
    onChanged: (next) => onChanged(next ?? ''),
  );
}

Future<void> _showGitHubActionComposer(
  BuildContext context,
  WorkspaceController controller,
) async {
  final repositoryController = TextEditingController();
  final issueController = TextEditingController();
  final titleController = TextEditingController();
  final bodyController = TextEditingController();
  final labelsController = TextEditingController();
  String operation = 'github.issue.create';

  final target = await showDialog<WorkspaceActionTarget>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (context, setState) {
        final createsIssue = operation == 'github.issue.create';
        final setsLabels = operation == 'github.issue.labels';
        return AlertDialog(
          title: const Text('Preview a GitHub action'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    'Only issue, comment and label actions are available. Push, merge and PR mutation are not exposed.',
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<String>(
                  initialValue: operation,
                  decoration: const InputDecoration(labelText: 'Action'),
                  items: const [
                    DropdownMenuItem(
                      value: 'github.issue.create',
                      child: Text('Create issue'),
                    ),
                    DropdownMenuItem(
                      value: 'github.issue.comment',
                      child: Text('Add issue comment'),
                    ),
                    DropdownMenuItem(
                      value: 'github.issue.labels',
                      child: Text('Set issue labels'),
                    ),
                  ],
                  onChanged: (value) =>
                      setState(() => operation = value ?? operation),
                ),
                const SizedBox(height: AppSpacing.md),
                TextField(
                  controller: repositoryController,
                  autofocus: true,
                  decoration: const InputDecoration(
                    labelText: 'Repository',
                    hintText: 'owner/repository',
                  ),
                ),
                if (!createsIssue) ...[
                  const SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: issueController,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(
                      labelText: 'Issue number',
                      hintText: '123',
                    ),
                  ),
                ],
                if (createsIssue) ...[
                  const SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: titleController,
                    decoration: const InputDecoration(labelText: 'Issue title'),
                  ),
                ],
                if (!setsLabels) ...[
                  const SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: bodyController,
                    minLines: 3,
                    maxLines: 6,
                    decoration: const InputDecoration(
                      labelText: 'Body / comment',
                      alignLabelWithHint: true,
                    ),
                  ),
                ],
                if (setsLabels) ...[
                  const SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: labelsController,
                    decoration: const InputDecoration(
                      labelText: 'Labels',
                      hintText: 'bug, needs-review',
                    ),
                  ),
                ],
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () {
                final repository = repositoryController.text.trim();
                final issue = int.tryParse(issueController.text.trim()) ?? 0;
                final title = titleController.text.trim();
                final body = bodyController.text.trim();
                final labels = labelsController.text
                    .split(',')
                    .map((label) => label.trim())
                    .where((label) => label.isNotEmpty)
                    .toList(growable: false);
                final valid =
                    repository.isNotEmpty &&
                    (createsIssue
                        ? title.isNotEmpty
                        : issue > 0 &&
                              (setsLabels
                                  ? labels.isNotEmpty
                                  : body.isNotEmpty));
                if (!valid) {
                  ScaffoldMessenger.of(dialogContext).showSnackBar(
                    const SnackBar(
                      content: Text(
                        'Điền repository và đúng trường bắt buộc cho action.',
                      ),
                    ),
                  );
                  return;
                }
                Navigator.pop(
                  dialogContext,
                  WorkspaceActionTarget(
                    operation: operation,
                    repository: repository,
                    issue: issue,
                    title: title,
                    body: body,
                    labels: labels,
                  ),
                );
              },
              child: const Text('Create preview'),
            ),
          ],
        );
      },
    ),
  );
  repositoryController.dispose();
  issueController.dispose();
  titleController.dispose();
  bodyController.dispose();
  labelsController.dispose();

  if (target == null || !context.mounted) return;
  final challenge = await controller.createActionChallenge(target);
  if (challenge == null || !context.mounted) return;
  final confirmed = await _showGitHubActionConfirmation(context, challenge);
  if (confirmed != true || !context.mounted) return;
  final result = await controller.confirmAction(challenge);
  if (result == null || !context.mounted) return;
  await _showGitHubReceipt(context, result);
}

Future<bool?> _showGitHubActionConfirmation(
  BuildContext context,
  WorkspaceActionChallenge challenge,
) => showDialog<bool>(
  context: context,
  builder: (dialogContext) => AlertDialog(
    title: const Text('Confirm exact GitHub action'),
    content: SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const StatusPill(
            label: 'Preview only until confirmed',
            color: AppColors.warning,
            icon: Icons.lock_outline,
          ),
          const SizedBox(height: AppSpacing.md),
          Text(
            _actionOperationLabel(challenge.target.operation),
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(challenge.target.repository),
          if (challenge.target.issue > 0) ...[
            const SizedBox(height: AppSpacing.xs),
            Text('Issue #${challenge.target.issue}'),
          ],
          if (challenge.target.title.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.md),
            Text(challenge.target.title),
          ],
          if (challenge.target.body.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.md),
            Text(challenge.target.body),
          ],
          if (challenge.target.labels.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.md),
            Text('Labels: ${challenge.target.labels.join(', ')}'),
          ],
          const SizedBox(height: AppSpacing.md),
          Text(
            'Challenge expires ${_actionExpiryLabel(challenge.expiresAt)}. Confirming sends this exact payload once; retry returns its original receipt.',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ],
      ),
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.pop(dialogContext, false),
        child: const Text('Cancel'),
      ),
      FilledButton(
        onPressed: () => Navigator.pop(dialogContext, true),
        child: const Text('Confirm and send'),
      ),
    ],
  ),
);

Future<void> _showGitHubReceipt(
  BuildContext context,
  WorkspaceActionConfirmation confirmation,
) => showDialog<void>(
  context: context,
  builder: (dialogContext) => AlertDialog(
    title: const Text('Action receipt'),
    content: Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const StatusPill(
          label: 'Recorded by backend',
          color: AppColors.success,
          icon: Icons.verified_outlined,
        ),
        const SizedBox(height: AppSpacing.md),
        Text(
          confirmation.replayed
              ? 'This was a safe replay of the original receipt.'
              : 'GitHub accepted the confirmed action.',
        ),
        const SizedBox(height: AppSpacing.sm),
        Text('Operation: ${confirmation.receipt.operation}'),
        Text('Status: ${confirmation.receipt.status}'),
        Text('Receipt ID: ${confirmation.receipt.id}'),
      ],
    ),
    actions: [
      FilledButton(
        onPressed: () => Navigator.pop(dialogContext),
        child: const Text('Done'),
      ),
    ],
  ),
);

String _actionOperationLabel(String operation) => switch (operation) {
  'github.issue.create' => 'Create GitHub issue',
  'github.issue.comment' => 'Add GitHub issue comment',
  'github.issue.labels' => 'Set GitHub issue labels',
  _ => operation,
};

String _actionExpiryLabel(DateTime? value) => value == null
    ? 'soon'
    : value.toLocal().toIso8601String().replaceFirst('T', ' ');

Future<void> _showWorkspaceAssistant(
  BuildContext context,
  WorkspaceController controller, {
  AppController? speechController,
  String initialPrompt = '',
  WorkspaceAssistantContext? assistantContext,
}) async {
  final messageController = TextEditingController(text: initialPrompt);
  try {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => AnimatedBuilder(
        animation: controller,
        builder: (context, _) => SafeArea(
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: MediaQuery.sizeOf(context).height * 0.86,
            ),
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.sm,
                AppSpacing.lg,
                AppSpacing.xl,
              ),
              child: _AssistantChat(
                controller: controller,
                messageController: messageController,
                speechController: speechController,
                onSubmit: ([value]) async {
                  final text = (value ?? messageController.text).trim();
                  if (text.isEmpty || controller.sending) return;
                  messageController.clear();
                  await controller.ask(text, context: assistantContext);
                },
              ),
            ),
          ),
        ),
      ),
    );
  } finally {
    messageController.dispose();
  }
}

Future<bool> _confirmTrash(BuildContext context, String title) async =>
    await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text('Move $title to trash?'),
        content: const Text(
          'The item is soft-deleted and can be restored during the retention window.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Move to trash'),
          ),
        ],
      ),
    ) ??
    false;

Future<void> _confirmTrashProject(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceProject project,
) async {
  if (await _confirmTrash(context, 'this project')) {
    await controller.trashProject(project);
  }
}

Future<void> _confirmTrashTask(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceTask task,
) async {
  if (await _confirmTrash(context, 'this task')) {
    await controller.trashTask(task);
  }
}

Future<void> _confirmTrashDecision(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceDecision decision,
) async {
  if (await _confirmTrash(context, 'this decision')) {
    await controller.trashDecision(decision);
  }
}

Future<bool> _confirmPurge(BuildContext context, String title) async =>
    await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text('Delete $title permanently?'),
        content: const Text(
          'This cannot be undone. Permanent deletion is intended only for an item already in trash after the retention policy is satisfied.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Delete permanently'),
          ),
        ],
      ),
    ) ??
    false;

Future<void> _confirmPurgeProject(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceProject project,
) async {
  if (await _confirmPurge(context, 'this project')) {
    await controller.purgeProject(project);
  }
}

Future<void> _confirmPurgeTask(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceTask task,
) async {
  if (await _confirmPurge(context, 'this task')) {
    await controller.purgeTask(task);
  }
}

Future<void> _confirmPurgeDecision(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceDecision decision,
) async {
  if (await _confirmPurge(context, 'this decision')) {
    await controller.purgeDecision(decision);
  }
}

String _enumValue(String value) =>
    value.trim().toLowerCase().replaceAll(' ', '_');

String _shortDate(DateTime value) =>
    '${value.year.toString().padLeft(4, '0')}-${value.month.toString().padLeft(2, '0')}-${value.day.toString().padLeft(2, '0')}';

Future<String?> _askForRepository(BuildContext context) async {
  final repositoryController = TextEditingController();
  final result = await showDialog<String>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Sync GitHub'),
      content: TextField(
        controller: repositoryController,
        autofocus: true,
        decoration: const InputDecoration(
          labelText: 'Repository',
          hintText: 'owner/repository',
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () =>
              Navigator.pop(dialogContext, repositoryController.text.trim()),
          child: const Text('Sync'),
        ),
      ],
    ),
  );
  repositoryController.dispose();
  return result;
}
