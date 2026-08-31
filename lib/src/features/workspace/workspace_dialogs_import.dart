part of 'workspace.dart';

Future<void> _showCreateProject(
  BuildContext context,
  WorkspaceController controller,
) async {
  final nameController = TextEditingController();
  final descriptionController = TextEditingController();
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('New project'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameController,
              autofocus: true,
              decoration: const InputDecoration(labelText: 'Name'),
            ),
            const SizedBox(height: AppSpacing.md),
            TextField(
              controller: descriptionController,
              maxLines: 3,
              decoration: const InputDecoration(labelText: 'Description'),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () async {
            final name = nameController.text.trim();
            if (name.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.createProject(
              name: name,
              description: descriptionController.text.trim(),
            );
          },
          child: const Text('Create'),
        ),
      ],
    ),
  );
  nameController.dispose();
  descriptionController.dispose();
}

Future<void> _showCreateTask(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceProject project,
) async {
  final titleController = TextEditingController();
  final descriptionController = TextEditingController();
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text('Add task to ${project.name}'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: titleController,
              autofocus: true,
              decoration: const InputDecoration(labelText: 'Title'),
            ),
            const SizedBox(height: AppSpacing.md),
            TextField(
              controller: descriptionController,
              maxLines: 3,
              decoration: const InputDecoration(labelText: 'Description'),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () async {
            final title = titleController.text.trim();
            if (title.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.createTask(
              projectId: project.id,
              title: title,
              description: descriptionController.text.trim(),
            );
          },
          child: const Text('Create'),
        ),
      ],
    ),
  );
  titleController.dispose();
  descriptionController.dispose();
}

Future<void> _showQuickCaptureTask(
  BuildContext context,
  WorkspaceController controller,
) async {
  final projects = controller.data.projects
      .where((project) => project.deletedAt == null)
      .toList(growable: false);
  if (projects.isEmpty) {
    final createProject = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Create a project first'),
        content: const Text(
          'A task must belong to an active project so it can be tracked in Work.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Create project'),
          ),
        ],
      ),
    );
    if (createProject == true && context.mounted) {
      await _showCreateProject(context, controller);
    }
    return;
  }

  var title = '';
  var description = '';
  var projectId = projects.length == 1 ? projects.single.id : '';
  var priority = 'normal';
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Quick capture task'),
      content: StatefulBuilder(
        builder: (context, setState) => SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                autofocus: true,
                onChanged: (value) => title = value,
                decoration: const InputDecoration(labelText: 'Task'),
              ),
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: projectId.isEmpty ? null : projectId,
                decoration: const InputDecoration(labelText: 'Project'),
                hint: const Text('Select a project'),
                items: [
                  for (final project in projects)
                    DropdownMenuItem(
                      value: project.id,
                      child: Text(project.name),
                    ),
                ],
                onChanged: (value) =>
                    setState(() => projectId = value ?? projectId),
              ),
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: priority,
                decoration: const InputDecoration(labelText: 'Priority'),
                items: const [
                  DropdownMenuItem(value: 'low', child: Text('Low')),
                  DropdownMenuItem(value: 'normal', child: Text('Normal')),
                  DropdownMenuItem(value: 'high', child: Text('High')),
                  DropdownMenuItem(value: 'urgent', child: Text('Urgent')),
                ],
                onChanged: (value) =>
                    setState(() => priority = value ?? priority),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                maxLines: 3,
                onChanged: (value) => description = value,
                decoration: const InputDecoration(
                  labelText: 'Description (optional)',
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () async {
            final taskTitle = title.trim();
            if (taskTitle.isEmpty || projectId.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.createTask(
              projectId: projectId,
              title: taskTitle,
              description: description.trim(),
              priority: priority,
            );
          },
          child: const Text('Create task'),
        ),
      ],
    ),
  );
}

Future<void> _showImportSource(
  BuildContext context,
  WorkspaceController controller,
) async {
  var draftName = '';
  var draftUri = '';
  var draftContent = '';
  final draft = await showDialog<Map<String, String>>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Draft knowledge source'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              autofocus: true,
              onChanged: (value) => draftName = value,
              decoration: const InputDecoration(labelText: 'Source name'),
            ),
            const SizedBox(height: AppSpacing.md),
            TextField(
              onChanged: (value) => draftUri = value,
              decoration: const InputDecoration(
                labelText: 'URI (optional)',
                hintText: 'manual://release-notes',
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            TextField(
              minLines: 5,
              maxLines: 10,
              onChanged: (value) => draftContent = value,
              decoration: const InputDecoration(
                labelText: 'Verified content',
                alignLabelWithHint: true,
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: () async {
            final name = draftName.trim();
            final content = draftContent.trim();
            if (name.isEmpty || content.isEmpty) return;
            Navigator.pop(dialogContext, <String, String>{
              'name': name,
              'content': content,
              'uri': draftUri.trim(),
            });
          },
          child: const Text('Review import'),
        ),
      ],
    ),
  );

  if (draft == null || !context.mounted) return;
  final name = draft['name'] ?? '';
  final content = draft['content'] ?? '';
  final uri = draft['uri'] ?? '';
  final confirmed = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Review source before import'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const StatusPill(
              label: 'Preview · no data written',
              color: AppColors.warning,
              icon: Icons.visibility_outlined,
            ),
            const SizedBox(height: AppSpacing.md),
            Text(name, style: Theme.of(context).textTheme.titleMedium),
            if (uri.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.xs),
              Text(uri, style: Theme.of(context).textTheme.bodySmall),
            ],
            const SizedBox(height: AppSpacing.md),
            Text(
              'This creates an immutable source revision and evidence only after you confirm.',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: AppSpacing.md),
            Container(
              constraints: const BoxConstraints(maxHeight: 220),
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: AppColors.surfaceMuted,
                borderRadius: BorderRadius.circular(AppRadius.small),
                border: Border.all(color: AppColors.border),
              ),
              child: SingleChildScrollView(child: SelectableText(content)),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(dialogContext, false),
          child: const Text('Back'),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(dialogContext, true),
          child: const Text('Import verified source'),
        ),
      ],
    ),
  );

  if (confirmed != true || !context.mounted) return;
  await controller.importManualSource(name: name, content: content, uri: uri);
}
