import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../theme.dart';
import '../workspace_controller.dart';
import '../workspace_models.dart';

class WorkspaceShell extends StatefulWidget {
  const WorkspaceShell({
    super.key,
    required this.controller,
    required this.learningController,
    required this.onPractice,
    required this.onReview,
    required this.onProgress,
    required this.onDiagnostic,
    required this.onRoleplay,
    required this.onCopilot,
    required this.onSettings,
  });

  final WorkspaceController controller;
  final AppController learningController;
  final VoidCallback onPractice;
  final VoidCallback onReview;
  final VoidCallback onProgress;
  final VoidCallback onDiagnostic;
  final VoidCallback onRoleplay;
  final VoidCallback onCopilot;
  final VoidCallback onSettings;

  @override
  State<WorkspaceShell> createState() => _WorkspaceShellState();
}

class _WorkspaceShellState extends State<WorkspaceShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 760;
        final pages = [
          TodayWorkspaceScreen(
            controller: widget.controller,
            onSettings: widget.onSettings,
          ),
          WorkWorkspaceScreen(data: widget.controller.data),
          KnowledgeWorkspaceScreen(data: widget.controller.data),
          LearningWorkspaceScreen(
            controller: widget.learningController,
            onPractice: widget.onPractice,
            onReview: widget.onReview,
            onProgress: widget.onProgress,
            onDiagnostic: widget.onDiagnostic,
            onRoleplay: widget.onRoleplay,
            onCopilot: widget.onCopilot,
          ),
        ];
        final content = IndexedStack(index: _index, children: pages);

        return Scaffold(
          body: Row(
            children: [
              if (!compact) _SideNavigation(index: _index, onSelected: _select),
              Expanded(child: content),
            ],
          ),
          bottomNavigationBar: compact
              ? NavigationBar(
                  selectedIndex: _index,
                  onDestinationSelected: _select,
                  destinations: _navigationDestinations,
                )
              : null,
        );
      },
    );
  }

  void _select(int index) => setState(() => _index = index);

  List<NavigationDestination> get _navigationDestinations => const [
    NavigationDestination(
      icon: Icon(Icons.today_outlined),
      selectedIcon: Icon(Icons.today),
      label: 'Today',
    ),
    NavigationDestination(
      icon: Icon(Icons.work_outline),
      selectedIcon: Icon(Icons.work),
      label: 'Work',
    ),
    NavigationDestination(
      icon: Icon(Icons.library_books_outlined),
      selectedIcon: Icon(Icons.library_books),
      label: 'Knowledge',
    ),
    NavigationDestination(
      icon: Icon(Icons.school_outlined),
      selectedIcon: Icon(Icons.school),
      label: 'Learning',
    ),
  ];
}

class _SideNavigation extends StatelessWidget {
  const _SideNavigation({required this.index, required this.onSelected});

  final int index;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 224,
      color: AppColors.surface,
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.xl,
                AppSpacing.xl,
                AppSpacing.lg,
                AppSpacing.lg,
              ),
              child: Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.accentSoft,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: const Icon(
                      Icons.auto_awesome,
                      color: AppColors.accent,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  const Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'DevEnglish',
                          style: TextStyle(fontWeight: FontWeight.w700),
                        ),
                        Text(
                          'Work assistant',
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.textSecondary,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: NavigationRail(
                selectedIndex: index,
                onDestinationSelected: onSelected,
                labelType: NavigationRailLabelType.all,
                groupAlignment: -0.9,
                destinations: const [
                  NavigationRailDestination(
                    icon: Icon(Icons.today_outlined),
                    selectedIcon: Icon(Icons.today),
                    label: Text('Today'),
                  ),
                  NavigationRailDestination(
                    icon: Icon(Icons.work_outline),
                    selectedIcon: Icon(Icons.work),
                    label: Text('Work'),
                  ),
                  NavigationRailDestination(
                    icon: Icon(Icons.library_books_outlined),
                    selectedIcon: Icon(Icons.library_books),
                    label: Text('Knowledge'),
                  ),
                  NavigationRailDestination(
                    icon: Icon(Icons.school_outlined),
                    selectedIcon: Icon(Icons.school),
                    label: Text('Learning'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class WorkspacePageFrame extends StatelessWidget {
  const WorkspacePageFrame({
    super.key,
    required this.title,
    required this.subtitle,
    required this.child,
    this.action,
  });

  final String title;
  final String subtitle;
  final Widget child;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final trailingActions = action == null ? <Widget>[] : <Widget>[action!];
    return SafeArea(
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.xl,
          AppSpacing.xl,
          AppSpacing.xl,
          AppSpacing.section,
        ),
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 1100),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            title,
                            style: Theme.of(context).textTheme.headlineSmall,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            subtitle,
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                        ],
                      ),
                    ),
                    ...trailingActions,
                  ],
                ),
                const SizedBox(height: AppSpacing.xxl),
                child,
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class WorkspacePreviewNotice extends StatelessWidget {
  const WorkspacePreviewNotice({super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.accentSoft,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.border),
      ),
      child: const Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, color: AppColors.accent),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              'Local preview: workspace data is read-only demo data until the production REST composition is connected.',
            ),
          ),
        ],
      ),
    );
  }
}

class TodayWorkspaceScreen extends StatefulWidget {
  const TodayWorkspaceScreen({
    super.key,
    required this.controller,
    required this.onSettings,
  });

  final WorkspaceController controller;
  final VoidCallback onSettings;

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
        action: IconButton(
          onPressed: widget.onSettings,
          tooltip: 'Settings',
          icon: const Icon(Icons.settings_outlined),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const WorkspacePreviewNotice(),
            const SizedBox(height: AppSpacing.xxl),
            const _TodaySummary(),
            const SizedBox(height: AppSpacing.section),
            const SectionTitle('Ask your assistant', trailing: 'Text-first'),
            const SizedBox(height: AppSpacing.md),
            _AssistantChat(
              controller: widget.controller,
              messageController: _messageController,
              onSubmit: _submit,
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

class _TodaySummary extends StatelessWidget {
  const _TodaySummary();

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: AppSpacing.md,
      runSpacing: AppSpacing.md,
      children: const [
        _SummaryCard(
          icon: Icons.task_alt,
          label: 'Focus',
          value: '2 open tasks',
          color: AppColors.accent,
        ),
        _SummaryCard(
          icon: Icons.menu_book_outlined,
          label: 'Knowledge',
          value: '1 source connected',
          color: AppColors.success,
        ),
        _SummaryCard(
          icon: Icons.language,
          label: 'English layer',
          value: 'B1 · practical',
          color: AppColors.warning,
        ),
      ],
    );
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
  Widget build(BuildContext context) {
    return SizedBox(
      width: 220,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: Row(
            children: [
              Icon(icon, color: color),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(label, style: Theme.of(context).textTheme.labelMedium),
                    const SizedBox(height: AppSpacing.xs),
                    Text(value, style: Theme.of(context).textTheme.titleMedium),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _AssistantChat extends StatelessWidget {
  const _AssistantChat({
    required this.controller,
    required this.messageController,
    required this.onSubmit,
  });

  final WorkspaceController controller;
  final TextEditingController messageController;
  final Future<void> Function([String? value]) onSubmit;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ...controller.conversation.map(
              (turn) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: AssistantTurnBubble(turn: turn),
              ),
            ),
            if (controller.sending) ...[
              const LinearProgressIndicator(minHeight: 3),
              const SizedBox(height: AppSpacing.md),
              const Text('Checking the connected source...'),
              const SizedBox(height: AppSpacing.md),
            ],
            if (controller.error != null) ...[
              Text(
                controller.error!,
                style: Theme.of(
                  context,
                ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
              ),
              const SizedBox(height: AppSpacing.md),
            ],
            TextField(
              key: const ValueKey('assistant-composer'),
              controller: messageController,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.send,
              onSubmitted: (value) => onSubmit(value),
              decoration: InputDecoration(
                hintText: 'Ask about a task, decision or source...',
                suffixIcon: IconButton(
                  tooltip: 'Send message',
                  onPressed: controller.sending ? null : () => onSubmit(),
                  icon: const Icon(Icons.arrow_upward_rounded),
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                _PromptChip(
                  label: 'What should I do next?',
                  onPressed: () => onSubmit('What should I do next?'),
                ),
                _PromptChip(
                  label: 'Show current tasks',
                  onPressed: () => onSubmit('Show my current tasks'),
                ),
                _PromptChip(
                  label: 'What is unknown?',
                  onPressed: () => onSubmit('What is unknown?'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _PromptChip extends StatelessWidget {
  const _PromptChip({required this.label, required this.onPressed});

  final String label;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) => ActionChip(
    label: Text(label),
    onPressed: onPressed,
    avatar: const Icon(Icons.arrow_forward, size: 16),
  );
}

class AssistantTurnBubble extends StatelessWidget {
  const AssistantTurnBubble({super.key, required this.turn});

  final AssistantTurn turn;

  @override
  Widget build(BuildContext context) {
    final isAssistant = turn.isAssistant;
    final bubbleColor = isAssistant
        ? AppColors.background
        : AppColors.accentSoft;
    return Align(
      alignment: isAssistant ? Alignment.centerLeft : Alignment.centerRight,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760),
        child: Container(
          padding: const EdgeInsets.all(AppSpacing.lg),
          decoration: BoxDecoration(
            color: bubbleColor,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: AppColors.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    isAssistant ? Icons.auto_awesome : Icons.person_outline,
                    size: 18,
                    color: isAssistant
                        ? AppColors.accent
                        : AppColors.textSecondary,
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Text(
                    isAssistant ? 'Assistant' : 'You',
                    style: Theme.of(context).textTheme.labelMedium,
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(turn.content, style: Theme.of(context).textTheme.bodyLarge),
              if (turn.unknowns.isNotEmpty) ...[
                const SizedBox(height: AppSpacing.md),
                _NoticeLine(
                  icon: Icons.help_outline,
                  color: AppColors.warning,
                  text: 'Unknown: ${turn.unknowns.join(' ')}',
                ),
              ],
              if (turn.suggestedActions.isNotEmpty) ...[
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Suggested next steps',
                  style: Theme.of(context).textTheme.labelMedium,
                ),
                const SizedBox(height: AppSpacing.sm),
                ...turn.suggestedActions.map(
                  (action) => Padding(
                    padding: const EdgeInsets.only(bottom: AppSpacing.xs),
                    child: Text('• $action'),
                  ),
                ),
              ],
              if (turn.citations.isNotEmpty) ...[
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Evidence',
                  style: Theme.of(context).textTheme.labelMedium,
                ),
                const SizedBox(height: AppSpacing.sm),
                ...turn.citations.map(CitationCard.new),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _NoticeLine extends StatelessWidget {
  const _NoticeLine({
    required this.icon,
    required this.color,
    required this.text,
  });

  final IconData icon;
  final Color color;
  final String text;

  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Icon(icon, size: 18, color: color),
      const SizedBox(width: AppSpacing.sm),
      Expanded(child: Text(text)),
    ],
  );
}

class CitationCard extends StatelessWidget {
  const CitationCard(this.citation, {super.key});

  final AssistantCitation citation;

  @override
  Widget build(BuildContext context) {
    final provenanceLabel = citation.origin.isPreview
        ? 'Demo preview'
        : citation.stale
        ? 'Stale source'
        : 'Source-backed';
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.surface,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              const Icon(Icons.link, size: 16, color: AppColors.success),
              Text(
                citation.sourceTitle,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              Chip(
                label: Text(provenanceLabel),
                visualDensity: VisualDensity.compact,
                side: BorderSide.none,
                backgroundColor: citation.origin.isPreview
                    ? AppColors.accentSoft
                    : citation.stale
                    ? const Color(0xFFFFF7ED)
                    : const Color(0xFFF0FDF4),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(
            citation.location,
            style: Theme.of(context).textTheme.labelMedium,
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(citation.excerpt),
        ],
      ),
    );
  }
}

class WorkWorkspaceScreen extends StatelessWidget {
  const WorkWorkspaceScreen({super.key, required this.data});

  final WorkspaceData data;

  @override
  Widget build(BuildContext context) {
    return WorkspacePageFrame(
      title: 'Work',
      subtitle: data.origin.isPreview
          ? 'Preview of projects, tasks and decisions.'
          : 'Projects, tasks and decisions stay canonical here.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const WorkspacePreviewNotice(),
          const SizedBox(height: AppSpacing.xxl),
          const SectionTitle('Projects'),
          const SizedBox(height: AppSpacing.md),
          ...data.projects.map((project) => _ProjectCard(project: project)),
          const SizedBox(height: AppSpacing.section),
          SectionTitle('Open tasks', trailing: data.origin.label),
          const SizedBox(height: AppSpacing.md),
          ...data.tasks.map(
            (task) => Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.md),
              child: _TaskCard(
                task: task,
                project: data.projectFor(task.projectId),
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Recent decisions'),
          const SizedBox(height: AppSpacing.md),
          ...data.decisions.map(
            (decision) => _DecisionCard(decision: decision),
          ),
        ],
      ),
    );
  }
}

class _ProjectCard extends StatelessWidget {
  const _ProjectCard({required this.project});

  final WorkspaceProject project;

  @override
  Widget build(BuildContext context) => Card(
    child: ListTile(
      contentPadding: const EdgeInsets.all(AppSpacing.lg),
      leading: const CircleAvatar(
        backgroundColor: AppColors.accentSoft,
        child: Icon(Icons.folder_open, color: AppColors.accent),
      ),
      title: Text(project.name, style: Theme.of(context).textTheme.titleMedium),
      subtitle: Padding(
        padding: const EdgeInsets.only(top: AppSpacing.xs),
        child: Text(project.summary),
      ),
      trailing: StatusChip(label: project.status, color: AppColors.success),
    ),
  );
}

class _TaskCard extends StatelessWidget {
  const _TaskCard({required this.task, required this.project});

  final WorkspaceTask task;
  final WorkspaceProject project;

  @override
  Widget build(BuildContext context) => Card(
    child: ListTile(
      contentPadding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.sm,
      ),
      leading: Icon(
        task.priority == 'High' ? Icons.priority_high : Icons.task_alt,
        color: task.priority == 'High' ? AppColors.warning : AppColors.accent,
      ),
      title: Text(task.title),
      subtitle: Text('${project.name} · ${task.status}'),
      trailing: StatusChip(label: task.priority, color: AppColors.warning),
    ),
  );
}

class _DecisionCard extends StatelessWidget {
  const _DecisionCard({required this.decision});

  final WorkspaceDecision decision;

  @override
  Widget build(BuildContext context) => Card(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    child: Padding(
      padding: const EdgeInsets.all(AppSpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(decision.title, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: AppSpacing.sm),
          Text(decision.outcome),
          const SizedBox(height: AppSpacing.sm),
          Text(
            decision.recordedAt,
            style: Theme.of(context).textTheme.labelMedium,
          ),
        ],
      ),
    ),
  );
}

class StatusChip extends StatelessWidget {
  const StatusChip({super.key, required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) => Chip(
    label: Text(label),
    visualDensity: VisualDensity.compact,
    side: BorderSide.none,
    backgroundColor: color.withValues(alpha: 0.1),
    labelStyle: TextStyle(color: color, fontSize: 12),
  );
}

class KnowledgeWorkspaceScreen extends StatefulWidget {
  const KnowledgeWorkspaceScreen({super.key, required this.data});

  final WorkspaceData data;

  @override
  State<KnowledgeWorkspaceScreen> createState() =>
      _KnowledgeWorkspaceScreenState();
}

class _KnowledgeWorkspaceScreenState extends State<KnowledgeWorkspaceScreen> {
  final _searchController = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final sources = widget.data.sources.where(_matches).toList();
    return WorkspacePageFrame(
      title: 'Knowledge',
      subtitle: widget.data.origin.isPreview
          ? 'Search preview evidence before asking the assistant to act.'
          : 'Search source-backed facts before asking the assistant to act.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const WorkspacePreviewNotice(),
          const SizedBox(height: AppSpacing.xxl),
          TextField(
            key: const ValueKey('knowledge-search'),
            controller: _searchController,
            onChanged: (value) =>
                setState(() => _query = value.trim().toLowerCase()),
            decoration: const InputDecoration(
              prefixIcon: Icon(Icons.search),
              hintText: 'Search sources, claims or evidence...',
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
          if (sources.isEmpty)
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
}

class _KnowledgeSourceCard extends StatelessWidget {
  const _KnowledgeSourceCard({required this.source, required this.isPreview});

  final KnowledgeSource source;
  final bool isPreview;

  @override
  Widget build(BuildContext context) => Card(
    margin: const EdgeInsets.only(bottom: AppSpacing.md),
    child: Padding(
      padding: const EdgeInsets.all(AppSpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Icon(Icons.description_outlined, color: AppColors.accent),
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
          Text('Evidence', style: Theme.of(context).textTheme.labelMedium),
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
        ],
      ),
    ),
  );
}

class _KnowledgeRulesCard extends StatelessWidget {
  const _KnowledgeRulesCard();

  @override
  Widget build(BuildContext context) => Card(
    color: AppColors.accentSoft,
    child: const Padding(
      padding: EdgeInsets.all(AppSpacing.lg),
      child: _NoticeLine(
        icon: Icons.shield_outlined,
        color: AppColors.accent,
        text:
            'Canonical facts need evidence. If no evidence is available, the assistant must say unknown or inferred.',
      ),
    ),
  );
}

class LearningWorkspaceScreen extends StatelessWidget {
  const LearningWorkspaceScreen({
    super.key,
    required this.controller,
    required this.onPractice,
    required this.onReview,
    required this.onProgress,
    required this.onDiagnostic,
    required this.onRoleplay,
    required this.onCopilot,
  });

  final AppController controller;
  final VoidCallback onPractice;
  final VoidCallback onReview;
  final VoidCallback onProgress;
  final VoidCallback onDiagnostic;
  final VoidCallback onRoleplay;
  final VoidCallback onCopilot;

  @override
  Widget build(BuildContext context) {
    final home = controller.home;
    return WorkspacePageFrame(
      title: 'Learning',
      subtitle: 'A quiet English layer around the work you already do.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const WorkspacePreviewNotice(),
          const SizedBox(height: AppSpacing.xxl),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Row(
                children: [
                  SizedBox(
                    width: 92,
                    height: 92,
                    child: Stack(
                      alignment: Alignment.center,
                      children: [
                        CircularProgressIndicator(
                          value: home.state.overallScore / 100,
                          strokeWidth: 8,
                          backgroundColor: AppColors.border,
                        ),
                        Text(
                          '${home.state.overallScore.round()}',
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
                        Text(
                          'Current English layer',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          '${home.state.cefr} · ${home.state.weakestSkill.replaceAll('_', ' ')} is the current focus.',
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        const Text(
                          'Learning observations stay separate from canonical work data.',
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Choose a focused practice'),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              OutlinedButton.icon(
                onPressed: onPractice,
                icon: const Icon(Icons.edit_note_rounded),
                label: const Text('Practice'),
              ),
              OutlinedButton.icon(
                onPressed: onReview,
                icon: const Icon(Icons.replay_outlined),
                label: const Text('Review'),
              ),
              OutlinedButton.icon(
                onPressed: onRoleplay,
                icon: const Icon(Icons.forum_outlined),
                label: const Text('Roleplay'),
              ),
              OutlinedButton.icon(
                onPressed: onCopilot,
                icon: const Icon(Icons.translate_outlined),
                label: const Text('English Copilot'),
              ),
              OutlinedButton.icon(
                onPressed: onProgress,
                icon: const Icon(Icons.insights_outlined),
                label: const Text('Progress'),
              ),
              OutlinedButton.icon(
                onPressed: onDiagnostic,
                icon: const Icon(Icons.fact_check_outlined),
                label: const Text('Diagnostic'),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xxl),
          LearningSupportCard(
            explanation:
                'Khi gặp một nhiệm vụ kỹ thuật, bạn chỉ cần nói ba ý: chuyện gì xảy ra, ảnh hưởng là gì và bước an toàn tiếp theo là gì.',
            starter:
                'The issue happens when ____. It affects ____. My next step is to ____.',
            followUp:
                'Hãy điền một chi tiết bạn có thể kiểm tra được vào mỗi chỗ trống.',
          ),
        ],
      ),
    );
  }
}
