import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../theme.dart';

class PracticeScreen extends StatelessWidget {
  const PracticeScreen({
    super.key,
    required this.controller,
    required this.onStartMission,
    required this.onWorkImport,
    required this.onSpeaking,
    required this.onRoleplay,
    required this.onCopilot,
    required this.onVocabulary,
  });

  final AppController controller;
  final VoidCallback onStartMission;
  final VoidCallback onWorkImport;
  final VoidCallback onSpeaking;
  final ValueChanged<String?> onRoleplay;
  final VoidCallback onCopilot;
  final VoidCallback onVocabulary;

  @override
  Widget build(BuildContext context) {
    return PageFrame(
      title: 'Practice',
      subtitle: 'Choose one clear task. Keep the session moving.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (controller.practice.isNotEmpty)
            ...controller.practice.map(
              (mode) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: Card(
                  child: ListTile(
                    minVerticalPadding: AppSpacing.md,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.lg,
                      vertical: AppSpacing.sm,
                    ),
                    leading: Icon(_iconFor(mode.id), color: AppColors.accent),
                    title: Text(
                      mode.title,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    subtitle: Padding(
                      padding: const EdgeInsets.only(top: AppSpacing.xs),
                      child: Text(
                        '${mode.description}\n${mode.minutes} minutes',
                      ),
                    ),
                    isThreeLine: true,
                    trailing: const Icon(Icons.chevron_right),
                    onTap: switch (mode.id) {
                      'writing' => onStartMission,
                      'speaking' => onSpeaking,
                      'roleplay' => () => onRoleplay(null),
                      'system-design' => () => onRoleplay('system-design'),
                      'technical-interview' => () => onRoleplay(
                        'technical-interview',
                      ),
                      'work-import' => onWorkImport,
                      'reading' => onVocabulary,
                      _ => () => _comingSoon(context, mode.title),
                    },
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  IconData _iconFor(String id) => switch (id) {
    'writing' => Icons.edit_note_rounded,
    'speaking' => Icons.mic_none_rounded,
    'roleplay' => Icons.forum_outlined,
    'system-design' => Icons.account_tree_outlined,
    'technical-interview' => Icons.question_answer_outlined,
    'reading' => Icons.menu_book_outlined,
    'work-import' => Icons.input_rounded,
    _ => Icons.school_outlined,
  };

  void _comingSoon(BuildContext context, String title) =>
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('$title is staged for the next vertical slice.'),
        ),
      );
}
