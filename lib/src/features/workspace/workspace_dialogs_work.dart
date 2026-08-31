part of 'workspace.dart';

Future<void> _showEditProject(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceProject project,
) async {
  final nameController = TextEditingController(text: project.name);
  final descriptionController = TextEditingController(text: project.summary);
  var status = _enumValue(project.status);
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Edit project'),
      content: StatefulBuilder(
        builder: (context, setState) => SingleChildScrollView(
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
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: status,
                decoration: const InputDecoration(labelText: 'Status'),
                items: const [
                  DropdownMenuItem(value: 'active', child: Text('Active')),
                  DropdownMenuItem(value: 'on_hold', child: Text('On hold')),
                  DropdownMenuItem(
                    value: 'completed',
                    child: Text('Completed'),
                  ),
                  DropdownMenuItem(value: 'archived', child: Text('Archived')),
                ],
                onChanged: (value) => setState(() => status = value ?? status),
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
            final name = nameController.text.trim();
            if (name.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.updateProject(
              project: project,
              name: name,
              description: descriptionController.text.trim(),
              status: status,
            );
          },
          child: const Text('Save'),
        ),
      ],
    ),
  );
  nameController.dispose();
  descriptionController.dispose();
}

Future<void> _showEditTask(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceTask task,
) async {
  final titleController = TextEditingController(text: task.title);
  final descriptionController = TextEditingController(text: task.description);
  var status = _enumValue(task.status);
  var priority = _enumValue(task.priority);
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Edit task'),
      content: StatefulBuilder(
        builder: (context, setState) => SingleChildScrollView(
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
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: status,
                decoration: const InputDecoration(labelText: 'Status'),
                items: const [
                  DropdownMenuItem(value: 'backlog', child: Text('Backlog')),
                  DropdownMenuItem(value: 'todo', child: Text('To do')),
                  DropdownMenuItem(
                    value: 'in_progress',
                    child: Text('In progress'),
                  ),
                  DropdownMenuItem(value: 'blocked', child: Text('Blocked')),
                  DropdownMenuItem(value: 'done', child: Text('Done')),
                  DropdownMenuItem(
                    value: 'cancelled',
                    child: Text('Cancelled'),
                  ),
                ],
                onChanged: (value) => setState(() => status = value ?? status),
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
            final title = titleController.text.trim();
            if (title.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.updateTask(
              task: task,
              title: title,
              description: descriptionController.text.trim(),
              status: status,
              priority: priority,
            );
          },
          child: const Text('Save'),
        ),
      ],
    ),
  );
  titleController.dispose();
  descriptionController.dispose();
}

Future<void> _showCreateDecision(
  BuildContext context,
  WorkspaceController controller,
) async {
  final titleController = TextEditingController();
  final contextController = TextEditingController();
  final outcomeController = TextEditingController();
  final rationaleController = TextEditingController();
  var projectId = '';
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Record decision'),
      content: StatefulBuilder(
        builder: (context, setState) => SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: titleController,
                autofocus: true,
                decoration: const InputDecoration(labelText: 'Decision'),
              ),
              const SizedBox(height: AppSpacing.md),
              _ProjectSelector(
                projects: controller.data.projects,
                value: projectId,
                onChanged: (value) => setState(() => projectId = value),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: contextController,
                maxLines: 2,
                decoration: const InputDecoration(labelText: 'Context'),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: outcomeController,
                maxLines: 3,
                decoration: const InputDecoration(labelText: 'Outcome'),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: rationaleController,
                maxLines: 3,
                decoration: const InputDecoration(labelText: 'Rationale'),
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
            final title = titleController.text.trim();
            final outcome = outcomeController.text.trim();
            if (title.isEmpty || outcome.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.createDecision(
              projectId: projectId,
              title: title,
              context: contextController.text.trim(),
              outcome: outcome,
              rationale: rationaleController.text.trim(),
            );
          },
          child: const Text('Record'),
        ),
      ],
    ),
  );
  titleController.dispose();
  contextController.dispose();
  outcomeController.dispose();
  rationaleController.dispose();
}

Future<void> _showEditDecision(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceDecision decision,
) async {
  final titleController = TextEditingController(text: decision.title);
  final contextController = TextEditingController(text: decision.context);
  final outcomeController = TextEditingController(text: decision.outcome);
  final rationaleController = TextEditingController(text: decision.rationale);
  var projectId = decision.projectId;
  var status = _enumValue(decision.status);
  await showDialog<void>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Edit decision'),
      content: StatefulBuilder(
        builder: (context, setState) => SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: titleController,
                autofocus: true,
                decoration: const InputDecoration(labelText: 'Decision'),
              ),
              const SizedBox(height: AppSpacing.md),
              _ProjectSelector(
                projects: controller.data.projects,
                value: projectId,
                onChanged: (value) => setState(() => projectId = value),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: contextController,
                maxLines: 2,
                decoration: const InputDecoration(labelText: 'Context'),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: outcomeController,
                maxLines: 3,
                decoration: const InputDecoration(labelText: 'Outcome'),
              ),
              const SizedBox(height: AppSpacing.md),
              TextField(
                controller: rationaleController,
                maxLines: 3,
                decoration: const InputDecoration(labelText: 'Rationale'),
              ),
              const SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: status,
                decoration: const InputDecoration(labelText: 'Status'),
                items: const [
                  DropdownMenuItem(value: 'proposed', child: Text('Proposed')),
                  DropdownMenuItem(value: 'accepted', child: Text('Accepted')),
                  DropdownMenuItem(value: 'rejected', child: Text('Rejected')),
                  DropdownMenuItem(
                    value: 'superseded',
                    child: Text('Superseded'),
                  ),
                ],
                onChanged: (value) => setState(() => status = value ?? status),
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
            final title = titleController.text.trim();
            final outcome = outcomeController.text.trim();
            if (title.isEmpty || outcome.isEmpty) return;
            Navigator.pop(dialogContext);
            await controller.updateDecision(
              decision: decision,
              projectId: projectId,
              title: title,
              context: contextController.text.trim(),
              outcome: outcome,
              rationale: rationaleController.text.trim(),
              status: status,
            );
          },
          child: const Text('Save'),
        ),
      ],
    ),
  );
  titleController.dispose();
  contextController.dispose();
  outcomeController.dispose();
  rationaleController.dispose();
}
