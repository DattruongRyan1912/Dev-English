import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../theme.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({
    super.key,
    required this.controller,
    required this.onStartMission,
    required this.onSettings,
    required this.onDiagnostic,
    required this.onCopilot,
    required this.onVocabulary,
  });

  final AppController controller;
  final VoidCallback onStartMission;
  final VoidCallback onSettings;
  final VoidCallback onDiagnostic;
  final VoidCallback onCopilot;
  final VoidCallback onVocabulary;

  @override
  Widget build(BuildContext context) {
    final home = controller.home;
    final weakest = home.state.skills.isEmpty
        ? null
        : home.state.skills.firstWhere(
            (skill) => skill.isWeakest,
            orElse: () => home.state.skills.first,
          );
    return PageFrame(
      title: 'Good morning, ${home.name}',
      subtitle: 'One focused step toward working in English.',
      action: IconButton(
        onPressed: onSettings,
        tooltip: 'Settings',
        icon: const Icon(Icons.settings_outlined),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (controller.error != null) ...[
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: AppColors.accentSoft,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Row(
                children: [
                  const Icon(Icons.info_outline, color: AppColors.accent),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      controller.error!,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: AppColors.textPrimary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.xl),
          ],
          const SectionTitle('Today\'s mission'),
          const SizedBox(height: AppSpacing.md),
          MissionBlock(mission: home.mission, onStart: onStartMission),
          const SizedBox(height: AppSpacing.section),
          SectionTitle('Review due', trailing: '${home.reviewDueCount} items'),
          const SizedBox(height: AppSpacing.md),
          const Card(
            child: ListTile(
              contentPadding: EdgeInsets.symmetric(
                horizontal: AppSpacing.lg,
                vertical: AppSpacing.sm,
              ),
              leading: CircleAvatar(
                backgroundColor: AppColors.accentSoft,
                child: Icon(Icons.replay_rounded, color: AppColors.accent),
              ),
              title: Text('Keep recent mistakes active'),
              subtitle: Text(
                'Short reviews are scheduled from your real work.',
              ),
              trailing: Icon(Icons.chevron_right),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          if (controller.diagnosticResult == null) ...[
            Card(
              child: ListTile(
                contentPadding: const EdgeInsets.all(AppSpacing.lg),
                leading: const Icon(
                  Icons.fact_check_outlined,
                  color: AppColors.accent,
                ),
                title: const Text('Set your starting point'),
                subtitle: const Text(
                  'Take the short diagnostic to tune the next missions.',
                ),
                trailing: const Icon(Icons.chevron_right),
                onTap: onDiagnostic,
              ),
            ),
            const SizedBox(height: AppSpacing.md),
          ],
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.lg),
              child: Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: [
                  OutlinedButton.icon(
                    onPressed: onCopilot,
                    icon: const Icon(Icons.translate_outlined),
                    label: const Text('English Copilot'),
                  ),
                  OutlinedButton.icon(
                    onPressed: onVocabulary,
                    icon: const Icon(Icons.menu_book_outlined),
                    label: const Text('Vocabulary'),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.section),
          const SectionTitle('Current focus'),
          const SizedBox(height: AppSpacing.md),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Developer English score',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text(
                        '${home.state.overallScore.round()}',
                        style: Theme.of(context).textTheme.displaySmall,
                      ),
                      const Padding(
                        padding: EdgeInsets.only(bottom: 4, left: 4),
                        child: Text('/ 100'),
                      ),
                    ],
                  ),
                  if (weakest != null) ...[
                    const SizedBox(height: AppSpacing.lg),
                    Text(
                      'Weakest skill: ${weakest.label}',
                      style: Theme.of(context).textTheme.bodyLarge,
                    ),
                    SkillBar(skill: weakest),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
