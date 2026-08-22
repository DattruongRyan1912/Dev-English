import 'package:flutter/material.dart';

import 'models.dart';
import 'theme.dart';

class PageFrame extends StatelessWidget {
  const PageFrame({
    super.key,
    required this.child,
    this.title,
    this.subtitle,
    this.action,
  });

  final Widget child;
  final String? title;
  final String? subtitle;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final heading = <Widget>[];
    if (title case final title?) {
      heading.add(
        Text(title, style: Theme.of(context).textTheme.headlineSmall),
      );
    }
    if (subtitle != null) {
      heading.add(const SizedBox(height: AppSpacing.sm));
      heading.add(
        Text(subtitle!, style: Theme.of(context).textTheme.bodyMedium),
      );
    }
    final trailingActions = <Widget>[];
    if (action case final action?) {
      trailingActions.add(action);
    }
    return SafeArea(
      child: CustomScrollView(
        slivers: [
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.xl,
              AppSpacing.xl,
              AppSpacing.lg,
            ),
            sliver: SliverToBoxAdapter(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: heading,
                    ),
                  ),
                  ...trailingActions,
                ],
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              0,
              AppSpacing.xl,
              AppSpacing.section,
            ),
            sliver: SliverToBoxAdapter(child: child),
          ),
        ],
      ),
    );
  }
}

class SectionTitle extends StatelessWidget {
  const SectionTitle(this.title, {super.key, this.trailing});

  final String title;
  final String? trailing;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Text(title, style: Theme.of(context).textTheme.titleMedium),
        ),
        if (trailing != null)
          Text(trailing!, style: Theme.of(context).textTheme.bodyMedium),
      ],
    );
  }
}

class SkillBar extends StatelessWidget {
  const SkillBar({super.key, required this.skill});

  final SkillState skill;

  @override
  Widget build(BuildContext context) {
    final color = skill.isWeakest ? AppColors.accent : AppColors.textSecondary;
    return Padding(
      padding: const EdgeInsets.only(top: AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  skill.label,
                  style: Theme.of(context).textTheme.bodyLarge,
                ),
              ),
              Text(
                '${skill.score.round()}%',
                style: Theme.of(
                  context,
                ).textTheme.titleMedium?.copyWith(color: color),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          ClipRRect(
            borderRadius: BorderRadius.circular(5),
            child: LinearProgressIndicator(
              value: skill.score / 100,
              minHeight: 7,
              backgroundColor: AppColors.border,
              color: color,
            ),
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(
            '${skill.trend >= 0 ? '+' : ''}${skill.trend.round()} this week',
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: skill.trend > 0
                  ? AppColors.success
                  : AppColors.textSecondary,
            ),
          ),
        ],
      ),
    );
  }
}

class MissionBlock extends StatelessWidget {
  const MissionBlock({super.key, required this.mission, required this.onStart});

  final Mission mission;
  final VoidCallback onStart;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 10,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.accentSoft,
                    borderRadius: BorderRadius.circular(99),
                  ),
                  child: Text(
                    '${mission.level} · ${mission.estimatedMinutes} min',
                    style: Theme.of(
                      context,
                    ).textTheme.labelMedium?.copyWith(color: AppColors.accent),
                  ),
                ),
                const Spacer(),
                const Icon(
                  Icons.edit_note_rounded,
                  color: AppColors.accent,
                  semanticLabel: 'Writing mission',
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(mission.title, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: AppSpacing.sm),
            Text(
              mission.context,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(mission.prompt, style: Theme.of(context).textTheme.bodyLarge),
            const SizedBox(height: AppSpacing.lg),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: mission.targetVocabulary
                  .map(
                    (term) => Chip(
                      label: Text(term),
                      visualDensity: VisualDensity.compact,
                      side: const BorderSide(color: AppColors.border),
                      backgroundColor: AppColors.background,
                    ),
                  )
                  .toList(),
            ),
            const SizedBox(height: AppSpacing.xl),
            FilledButton(
              onPressed: onStart,
              child: const Text('Start mission'),
            ),
          ],
        ),
      ),
    );
  }
}

class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.title,
    required this.description,
    this.action,
  });

  final String title;
  final String description;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 48),
        child: Column(
          children: [
            const Icon(
              Icons.check_circle_outline,
              size: 40,
              color: AppColors.success,
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(
              title,
              style: Theme.of(context).textTheme.titleMedium,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              description,
              style: Theme.of(context).textTheme.bodyMedium,
              textAlign: TextAlign.center,
            ),
            if (action != null) ...[
              const SizedBox(height: AppSpacing.lg),
              action!,
            ],
          ],
        ),
      ),
    );
  }
}

class FocusHeader extends StatelessWidget {
  const FocusHeader({super.key, required this.title, required this.onExit});

  final String title;
  final VoidCallback onExit;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        IconButton(
          onPressed: onExit,
          tooltip: 'Exit mission',
          icon: const Icon(Icons.close),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: Text(title, style: Theme.of(context).textTheme.titleMedium),
        ),
        Text('Writing', style: Theme.of(context).textTheme.labelMedium),
      ],
    );
  }
}
