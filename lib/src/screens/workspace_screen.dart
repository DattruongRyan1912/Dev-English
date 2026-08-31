import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../features/workspace/workspace.dart';
import '../theme.dart';
import '../workspace_controller.dart';

export '../features/workspace/workspace.dart'
    show
        AssistantTurnBubble,
        CitationCard,
        KnowledgeWorkspaceScreen,
        LearningWorkspaceScreen,
        TodayWorkspaceScreen,
        WorkWorkspaceScreen,
        WorkspacePageFrame,
        WorkspacePreviewNotice;

/// Compatibility implementation for callers that still import the old
/// learning-first screen file. New app routing uses
/// `src/app/workspace_shell.dart`.
class LegacyWorkspaceShell extends StatefulWidget {
  const LegacyWorkspaceShell({
    super.key,
    required this.controller,
    required this.learningController,
    required this.onPractice,
    required this.onReview,
    required this.onProgress,
    required this.onDiagnostic,
    required this.onRoleplay,
    required this.onCopilot,
    required this.onSpeaking,
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
  final VoidCallback onSpeaking;
  final VoidCallback onSettings;

  @override
  State<LegacyWorkspaceShell> createState() => _LegacyWorkspaceShellState();
}

class _LegacyWorkspaceShellState extends State<LegacyWorkspaceShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: widget.controller,
      builder: (context, _) => LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 760;
          final liveController = widget.controller.canonicalLoaded
              ? widget.controller
              : null;
          final pages = [
            TodayWorkspaceScreen(
              controller: widget.controller,
              onSettings: widget.onSettings,
            ),
            WorkWorkspaceScreen(
              data: widget.controller.data,
              controller: liveController,
            ),
            KnowledgeWorkspaceScreen(
              data: widget.controller.data,
              controller: liveController,
            ),
            LearningWorkspaceScreen(
              controller: widget.learningController,
              workspaceController: widget.controller,
              onPractice: widget.onPractice,
              onReview: widget.onReview,
              onProgress: widget.onProgress,
              onDiagnostic: widget.onDiagnostic,
              onRoleplay: widget.onRoleplay,
              onCopilot: widget.onCopilot,
              onSpeaking: widget.onSpeaking,
            ),
          ];
          final content = IndexedStack(index: _index, children: pages);

          return Scaffold(
            body: Row(
              children: [
                if (!compact)
                  _SideNavigation(index: _index, onSelected: _select),
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
      ),
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
      width: 240,
      decoration: const BoxDecoration(
        color: AppColors.surface,
        border: Border(right: BorderSide(color: AppColors.border)),
      ),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.xxl,
                AppSpacing.xxl,
                AppSpacing.xl,
                AppSpacing.section,
              ),
              child: Row(
                children: [
                  const AppIconBadge(icon: Icons.auto_awesome),
                  const SizedBox(width: AppSpacing.md),
                  const Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'DevEnglish',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
                        ),
                        Text(
                          'Work companion',
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
              child: ListView(
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
                children: [
                  const Padding(
                    padding: EdgeInsets.fromLTRB(
                      AppSpacing.md,
                      0,
                      AppSpacing.md,
                      AppSpacing.sm,
                    ),
                    child: EyebrowLabel('Workspace'),
                  ),
                  _WorkspaceNavItem(
                    icon: Icons.today_outlined,
                    selectedIcon: Icons.today,
                    label: 'Today',
                    selected: index == 0,
                    onTap: () => onSelected(0),
                  ),
                  _WorkspaceNavItem(
                    icon: Icons.work_outline,
                    selectedIcon: Icons.work,
                    label: 'Work',
                    selected: index == 1,
                    onTap: () => onSelected(1),
                  ),
                  _WorkspaceNavItem(
                    icon: Icons.library_books_outlined,
                    selectedIcon: Icons.library_books,
                    label: 'Knowledge',
                    selected: index == 2,
                    onTap: () => onSelected(2),
                  ),
                  _WorkspaceNavItem(
                    icon: Icons.school_outlined,
                    selectedIcon: Icons.school,
                    label: 'Learning',
                    selected: index == 3,
                    onTap: () => onSelected(3),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.xxl,
                AppSpacing.lg,
                AppSpacing.xxl,
                AppSpacing.xxl,
              ),
              child: const Row(
                children: [
                  Icon(Icons.circle, size: 8, color: AppColors.success),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      'Local workspace',
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.textSecondary,
                      ),
                    ),
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

class _WorkspaceNavItem extends StatelessWidget {
  const _WorkspaceNavItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final foreground = selected
        ? AppColors.accentStrong
        : AppColors.textSecondary;
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.xs),
      child: Semantics(
        button: true,
        selected: selected,
        label: label,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(AppRadius.small),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 180),
              constraints: const BoxConstraints(minHeight: 48),
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
              decoration: BoxDecoration(
                color: selected ? AppColors.accentSoft : Colors.transparent,
                borderRadius: BorderRadius.circular(AppRadius.small),
              ),
              child: Row(
                children: [
                  Icon(
                    selected ? selectedIcon : icon,
                    color: foreground,
                    size: 21,
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Text(
                    label,
                    style: Theme.of(context).textTheme.labelLarge?.copyWith(
                      color: foreground,
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
