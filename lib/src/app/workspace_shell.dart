import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../features/knowledge/knowledge_screen.dart';
import '../features/learning/learning_screen.dart';
import '../features/today/today_screen.dart';
import '../features/work/work_screen.dart';
import '../theme.dart';
import '../workspace_controller.dart';

/// Product-reset application shell.
///
/// The shell owns navigation and responsive composition. Feature
/// implementations are split into the screen part files and exposed through
/// feature boundaries. The shell imports the owning library once because Dart
/// part files cannot be imported as standalone libraries.
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
  State<WorkspaceShell> createState() => _WorkspaceShellState();
}

class _WorkspaceShellState extends State<WorkspaceShell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: widget.controller,
      builder: (context, _) {
        if (widget.controller.hasWorkspaceApi &&
            !widget.controller.canonicalLoaded) {
          return _WorkspaceUnavailableState(
            loading: widget.controller.loading,
            error: widget.controller.error,
            onRetry: widget.controller.load,
          );
        }
        return LayoutBuilder(
          builder: (context, constraints) {
            final compact = constraints.maxWidth < 760;
            final liveController = widget.controller.canonicalLoaded
                ? widget.controller
                : null;
            final pages = [
              TodayWorkspaceScreen(
                controller: widget.controller,
                onSettings: widget.onSettings,
                speechController: widget.learningController,
              ),
              WorkWorkspaceScreen(
                data: widget.controller.data,
                controller: liveController,
                speechController: widget.learningController,
              ),
              KnowledgeWorkspaceScreen(
                data: widget.controller.data,
                controller: liveController,
                speechController: widget.learningController,
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

            return Scaffold(
              body: Row(
                children: [
                  if (!compact)
                    _WorkspaceSideNavigation(
                      index: _index,
                      onSelected: _select,
                    ),
                  Expanded(
                    child: IndexedStack(index: _index, children: pages),
                  ),
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
      },
    );
  }

  void _select(int index) => setState(() => _index = index);

  static const _navigationDestinations = <NavigationDestination>[
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

class _WorkspaceUnavailableState extends StatelessWidget {
  const _WorkspaceUnavailableState({
    required this.loading,
    required this.error,
    required this.onRetry,
  });

  final bool loading;
  final String? error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final waiting = loading && error == null;
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.xxl),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 520),
              child: AppSurface(
                color: AppColors.surfaceAccent,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      waiting ? Icons.sync_outlined : Icons.cloud_off_outlined,
                      size: 48,
                      color: waiting ? AppColors.accent : AppColors.warning,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    Text(
                      waiting
                          ? 'Loading your workspace'
                          : 'Workspace is temporarily unavailable',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      waiting
                          ? 'Connecting to the canonical backend data. Please wait.'
                          : (error ??
                                'The canonical workspace could not be loaded.'),
                      textAlign: TextAlign.center,
                    ),
                    if (waiting) ...[
                      const SizedBox(height: AppSpacing.lg),
                      const SizedBox(
                        width: 220,
                        child: LinearProgressIndicator(),
                      ),
                    ] else ...[
                      const SizedBox(height: AppSpacing.lg),
                      FilledButton.icon(
                        onPressed: onRetry,
                        icon: const Icon(Icons.refresh),
                        label: const Text('Retry connection'),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _WorkspaceSideNavigation extends StatelessWidget {
  const _WorkspaceSideNavigation({
    required this.index,
    required this.onSelected,
  });

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
              child: const Row(
                children: [
                  AppIconBadge(icon: Icons.auto_awesome),
                  SizedBox(width: AppSpacing.md),
                  Expanded(
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
            const Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.xxl,
                AppSpacing.lg,
                AppSpacing.xxl,
                AppSpacing.xxl,
              ),
              child: Row(
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
