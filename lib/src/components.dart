import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'models.dart';
import 'theme.dart';

class AppSurface extends StatelessWidget {
  const AppSurface({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(AppSpacing.lg),
    this.color = AppColors.surface,
    this.borderColor = AppColors.border,
    this.radius = AppRadius.medium,
    this.margin,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final Color color;
  final Color borderColor;
  final double radius;
  final EdgeInsetsGeometry? margin;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: margin,
      padding: padding,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(radius),
        border: Border.all(color: borderColor),
      ),
      child: child,
    );
  }
}

class AppIconBadge extends StatelessWidget {
  const AppIconBadge({
    super.key,
    required this.icon,
    this.color = AppColors.accent,
    this.backgroundColor = AppColors.accentSoft,
    this.size = 40,
  });

  final IconData icon;
  final Color color;
  final Color backgroundColor;
  final double size;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(AppRadius.small),
      ),
      child: Icon(icon, color: color, size: size * 0.52),
    );
  }
}

class StatusPill extends StatelessWidget {
  const StatusPill({
    super.key,
    required this.label,
    required this.color,
    this.icon,
  });

  final String label;
  final Color color;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: 28),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 14, color: color),
            const SizedBox(width: 5),
          ],
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 240),
            child: Text(
              label,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.labelMedium?.copyWith(
                color: color,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class EyebrowLabel extends StatelessWidget {
  const EyebrowLabel(
    this.text, {
    super.key,
    this.color = AppColors.textTertiary,
  });

  final String text;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Text(
      text.toUpperCase(),
      style: Theme.of(context).textTheme.labelMedium?.copyWith(
        color: color,
        fontWeight: FontWeight.w700,
        letterSpacing: 1.1,
      ),
    );
  }
}

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
          Flexible(
            fit: FlexFit.loose,
            child: Text(
              trailing!,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.end,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
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

class HoldToTalkButton extends StatefulWidget {
  const HoldToTalkButton({
    super.key,
    required this.enabled,
    required this.recording,
    required this.onPressStart,
    required this.onPressEnd,
    required this.onPressCancel,
  });

  final bool enabled;
  final bool recording;
  final VoidCallback onPressStart;
  final VoidCallback onPressEnd;
  final VoidCallback onPressCancel;

  @override
  State<HoldToTalkButton> createState() => _HoldToTalkButtonState();
}

class _HoldToTalkButtonState extends State<HoldToTalkButton> {
  var _keyboardPressActive = false;
  var _hasFocus = false;

  @override
  void didUpdateWidget(covariant HoldToTalkButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!widget.enabled && _keyboardPressActive) {
      _keyboardPressActive = false;
      widget.onPressCancel();
    }
  }

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (!widget.enabled ||
        (event.logicalKey != LogicalKeyboardKey.space &&
            event.logicalKey != LogicalKeyboardKey.enter)) {
      return KeyEventResult.ignored;
    }

    if (event is KeyDownEvent) {
      if (!_keyboardPressActive && !widget.recording) {
        _keyboardPressActive = true;
        widget.onPressStart();
      }
      return KeyEventResult.handled;
    }
    if (event is KeyUpEvent) {
      if (_keyboardPressActive) {
        _keyboardPressActive = false;
        widget.onPressEnd();
      }
      return KeyEventResult.handled;
    }
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final background = widget.recording ? AppColors.warning : AppColors.accent;
    return Semantics(
      button: true,
      enabled: widget.enabled,
      label: widget.recording ? 'Release to transcribe' : 'Hold to talk',
      hint:
          'Press and hold while speaking, then release to transcribe. You can also hold Space or Enter.',
      child: Focus(
        key: const ValueKey('hold-to-talk-focus'),
        canRequestFocus: widget.enabled,
        skipTraversal: !widget.enabled,
        onKeyEvent: _handleKeyEvent,
        onFocusChange: (focused) {
          if (!focused && _keyboardPressActive) {
            _keyboardPressActive = false;
            widget.onPressCancel();
          }
          if (mounted) setState(() => _hasFocus = focused);
        },
        child: GestureDetector(
          key: const ValueKey('hold-to-talk'),
          behavior: HitTestBehavior.opaque,
          onTapDown: widget.enabled ? (_) => widget.onPressStart() : null,
          onTapUp: widget.enabled ? (_) => widget.onPressEnd() : null,
          onTapCancel: widget.enabled ? widget.onPressCancel : null,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 150),
            width: double.infinity,
            constraints: const BoxConstraints(minHeight: 48),
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.lg,
              vertical: AppSpacing.md,
            ),
            decoration: BoxDecoration(
              color: widget.enabled ? background : AppColors.border,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: _hasFocus ? AppColors.textPrimary : Colors.transparent,
                width: 2,
              ),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  widget.recording
                      ? Icons.stop_circle_outlined
                      : Icons.mic_none,
                  color: widget.enabled
                      ? Colors.white
                      : AppColors.textSecondary,
                ),
                const SizedBox(width: AppSpacing.sm),
                Text(
                  widget.recording ? 'Release to transcribe' : 'Hold to talk',
                  style: Theme.of(context).textTheme.labelLarge?.copyWith(
                    color: widget.enabled
                        ? Colors.white
                        : AppColors.textSecondary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class LearningSupportCard extends StatelessWidget {
  const LearningSupportCard({
    super.key,
    required this.explanation,
    required this.starter,
    required this.followUp,
    this.title = 'Need a little help?',
    this.onSpeak,
  });

  final String title;
  final String explanation;
  final String starter;
  final String followUp;
  final ValueChanged<String>? onSpeak;

  @override
  Widget build(BuildContext context) {
    return AppSurface(
      color: AppColors.accentSoft,
      padding: EdgeInsets.zero,
      child: Material(
        color: Colors.transparent,
        child: ExpansionTile(
          key: const ValueKey('learning-overlay'),
          leading: const Icon(Icons.lightbulb_outline, color: AppColors.accent),
          title: Text(title),
          subtitle: const Text(
            'Short Vietnamese guidance and one English starter. Open only when needed.',
          ),
          childrenPadding: const EdgeInsets.fromLTRB(
            AppSpacing.xl,
            0,
            AppSpacing.xl,
            AppSpacing.lg,
          ),
          children: [
            _SupportBlock(title: 'Giải thích ngắn', body: explanation),
            const SizedBox(height: AppSpacing.md),
            _SupportBlock(
              title: 'English starter',
              body: starter,
              action: onSpeak == null
                  ? null
                  : IconButton(
                      onPressed: () => onSpeak!(starter),
                      tooltip: 'Listen to English starter',
                      icon: const Icon(Icons.volume_up_outlined),
                    ),
            ),
            const SizedBox(height: AppSpacing.md),
            _SupportBlock(title: 'Try this next', body: followUp),
          ],
        ),
      ),
    );
  }
}

class _SupportBlock extends StatelessWidget {
  const _SupportBlock({required this.title, required this.body, this.action});

  final String title;
  final String body;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: Theme.of(context).textTheme.labelLarge),
              const SizedBox(height: AppSpacing.xs),
              Text(body, style: Theme.of(context).textTheme.bodyMedium),
            ],
          ),
        ),
        action ?? const SizedBox.shrink(),
      ],
    );
  }
}
