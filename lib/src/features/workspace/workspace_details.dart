part of 'workspace.dart';

Future<void> _showProjectDetails(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceProject project,
) async {
  final linkedTasks = controller.data.tasks
      .where((task) => task.projectId == project.id)
      .map((task) => '${task.title} · ${task.status}')
      .toList(growable: false);
  final linkedDecisions = controller.data.decisions
      .where((decision) => decision.projectId == project.id)
      .map((decision) => '${decision.title} · ${decision.status}')
      .toList(growable: false);
  await _showWorkEntityDetails(
    context,
    entityLabel: 'Project',
    title: project.name,
    fields: [
      _WorkDetailField('Status', project.status),
      _WorkDetailField('Description', project.summary),
      _WorkDetailField('Version', '${project.version}'),
    ],
    sections: [
      _WorkDetailSection(
        title: 'Tasks',
        items: linkedTasks,
        emptyText: 'No tasks are linked to this project yet.',
      ),
      _WorkDetailSection(
        title: 'Decisions',
        items: linkedDecisions,
        emptyText: 'No decisions are linked to this project yet.',
      ),
      const _WorkDetailSection(
        title: 'Linked knowledge',
        items: [],
        emptyText: 'No project-linked knowledge is recorded yet.',
      ),
    ],
    history: controller.projectHistory(project.id),
  );
}

Future<void> _showTaskDetails(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceTask task,
  WorkspaceProject project,
) async {
  await _showWorkEntityDetails(
    context,
    entityLabel: 'Task',
    title: task.title,
    fields: [
      _WorkDetailField('Project', project.name),
      _WorkDetailField('Status', task.status),
      _WorkDetailField('Priority', task.priority),
      _WorkDetailField(
        'Description',
        task.description.isEmpty ? 'No description yet.' : task.description,
      ),
      _WorkDetailField('Due date', _workDate(task.dueAt) ?? 'No due date set.'),
      _WorkDetailField('Version', '${task.version}'),
    ],
    sections: const [
      _WorkDetailSection(
        title: 'Conversation',
        items: [],
        emptyText:
            'Task-specific conversation will appear here after this task is used as assistant context.',
      ),
    ],
    history: controller.taskHistory(task.id),
  );
}

Future<void> _showDecisionDetails(
  BuildContext context,
  WorkspaceController controller,
  WorkspaceDecision decision,
) async {
  await _showWorkEntityDetails(
    context,
    entityLabel: 'Decision',
    title: decision.title,
    fields: [
      _WorkDetailField('Status', decision.status),
      _WorkDetailField('Outcome', decision.outcome),
      _WorkDetailField(
        'Context',
        decision.context.isEmpty ? 'No context recorded.' : decision.context,
      ),
      _WorkDetailField(
        'Rationale',
        decision.rationale.isEmpty
            ? 'No rationale recorded.'
            : decision.rationale,
      ),
      _WorkDetailField('Version', '${decision.version}'),
    ],
    sections: const [
      _WorkDetailSection(
        title: 'Alternatives',
        items: [],
        emptyText: 'No alternatives are recorded for this decision yet.',
      ),
      _WorkDetailSection(
        title: 'Evidence',
        items: [],
        emptyText:
            'No evidence is linked to this decision yet. Treat the outcome as unverified.',
      ),
    ],
    history: controller.decisionHistory(decision.id),
  );
}

Future<void> _showWorkEntityDetails(
  BuildContext context, {
  required String entityLabel,
  required String title,
  required List<_WorkDetailField> fields,
  List<_WorkDetailSection> sections = const [],
  required Future<List<WorkspaceHistoryEvent>> history,
}) async {
  await showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (context) => _WorkEntityDetailsSheet(
      entityLabel: entityLabel,
      title: title,
      fields: fields,
      sections: sections,
      history: history,
    ),
  );
}

class _WorkDetailField {
  const _WorkDetailField(this.label, this.value);

  final String label;
  final String value;
}

class _WorkDetailSection {
  const _WorkDetailSection({
    required this.title,
    required this.items,
    required this.emptyText,
  });

  final String title;
  final List<String> items;
  final String emptyText;
}

class _WorkEntityDetailsSheet extends StatelessWidget {
  const _WorkEntityDetailsSheet({
    required this.entityLabel,
    required this.title,
    required this.fields,
    required this.sections,
    required this.history,
  });

  final String entityLabel;
  final String title;
  final List<_WorkDetailField> fields;
  final List<_WorkDetailSection> sections;
  final Future<List<WorkspaceHistoryEvent>> history;

  @override
  Widget build(BuildContext context) => SafeArea(
    child: FractionallySizedBox(
      key: const ValueKey('work-details-sheet'),
      heightFactor: 0.9,
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.xl,
          AppSpacing.sm,
          AppSpacing.xl,
          AppSpacing.xxl,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            EyebrowLabel('$entityLabel details'),
            const SizedBox(height: AppSpacing.sm),
            Text(title, style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: AppSpacing.xl),
            ...fields.map(
              (field) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: _WorkDetailValue(field: field),
              ),
            ),
            if (sections.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.sm),
              ...sections.map(
                (section) => Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.lg),
                  child: _WorkDetailSectionView(section: section),
                ),
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            const Divider(),
            const SizedBox(height: AppSpacing.lg),
            const SectionTitle('Activity history'),
            const SizedBox(height: AppSpacing.sm),
            FutureBuilder<List<WorkspaceHistoryEvent>>(
              future: history,
              builder: (context, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Padding(
                    padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
                    child: Center(child: CircularProgressIndicator()),
                  );
                }
                final events = snapshot.data ?? const [];
                if (events.isEmpty) {
                  return const Text('No activity history available yet.');
                }
                return Column(
                  children: [
                    for (var index = 0; index < events.length; index++)
                      _HistoryEventTile(
                        event: events[index],
                        isLast: index == events.length - 1,
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    ),
  );
}

class _WorkDetailValue extends StatelessWidget {
  const _WorkDetailValue({required this.field});

  final _WorkDetailField field;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      EyebrowLabel(field.label),
      const SizedBox(height: AppSpacing.xs),
      Text(field.value, style: Theme.of(context).textTheme.bodyLarge),
    ],
  );
}

class _WorkDetailSectionView extends StatelessWidget {
  const _WorkDetailSectionView({required this.section});

  final _WorkDetailSection section;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(section.title, style: Theme.of(context).textTheme.titleSmall),
      const SizedBox(height: AppSpacing.sm),
      if (section.items.isEmpty)
        Text(section.emptyText, style: Theme.of(context).textTheme.bodyMedium)
      else
        ...section.items.map(
          (item) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.xs),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Padding(
                  padding: EdgeInsets.only(top: 6),
                  child: Icon(Icons.circle, size: 6, color: AppColors.accent),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(child: Text(item)),
              ],
            ),
          ),
        ),
    ],
  );
}

class _HistoryEventTile extends StatelessWidget {
  const _HistoryEventTile({required this.event, required this.isLast});

  final WorkspaceHistoryEvent event;
  final bool isLast;

  @override
  Widget build(BuildContext context) => IntrinsicHeight(
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(
          width: 28,
          child: Column(
            children: [
              Icon(_historyIcon(event.action), color: AppColors.accent),
              if (!isLast)
                Expanded(
                  child: Container(
                    width: 1,
                    color: AppColors.border,
                    margin: const EdgeInsets.symmetric(vertical: 4),
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.lg),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  event.action,
                  style: Theme.of(context).textTheme.titleSmall,
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  'Version ${event.fromVersion} → ${event.toVersion} · ${_historyDate(event.createdAt)}',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                if (event.actorUserId.isNotEmpty) ...[
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    'Actor ${event.actorUserId}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ],
              ],
            ),
          ),
        ),
      ],
    ),
  );
}

IconData _historyIcon(String action) => switch (action.toLowerCase()) {
  'created' => Icons.add_circle_outline,
  'updated' => Icons.edit_outlined,
  'trashed' => Icons.delete_outline,
  'restored' => Icons.restore_outlined,
  'purged' => Icons.delete_forever_outlined,
  _ => Icons.history,
};

String _historyDate(DateTime? value) {
  if (value == null) return 'Time unavailable';
  final local = value.toLocal();
  String twoDigits(int number) => number.toString().padLeft(2, '0');
  return '${local.year}-${twoDigits(local.month)}-${twoDigits(local.day)} '
      '${twoDigits(local.hour)}:${twoDigits(local.minute)}';
}

String? _workDate(DateTime? value) {
  if (value == null) return null;
  final local = value.toLocal();
  String twoDigits(int number) => number.toString().padLeft(2, '0');
  return '${local.year}-${twoDigits(local.month)}-${twoDigits(local.day)}';
}
