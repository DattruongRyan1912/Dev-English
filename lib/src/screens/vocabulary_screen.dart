import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../components.dart';
import '../theme.dart';

class VocabularyScreen extends StatefulWidget {
  const VocabularyScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<VocabularyScreen> createState() => _VocabularyScreenState();
}

class _VocabularyScreenState extends State<VocabularyScreen> {
  @override
  void initState() {
    super.initState();
    widget.controller.loadVocabularyAndUsage();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: AppColors.background,
    appBar: AppBar(
      title: const Text('Technical Vocabulary'),
      backgroundColor: AppColors.background,
      surfaceTintColor: Colors.transparent,
    ),
    body: SafeArea(
      child: AnimatedBuilder(
        animation: widget.controller,
        builder: (context, _) {
          final items = widget.controller.vocabulary;
          if (items.isEmpty) {
            return const EmptyState(
              title: 'No vocabulary yet',
              description:
                  'Import work context to build a personal technical vocabulary graph.',
            );
          }
          final graph = widget.controller.vocabularyGraph;
          final labels = {for (final node in graph.nodes) node.id: node.label};
          return ListView(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.md,
              AppSpacing.xl,
              AppSpacing.section,
            ),
            children: [
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.lg),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Personal vocabulary graph',
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        '${graph.nodes.length} terms · ${graph.edges.length} related connections',
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                      if (graph.edges.isNotEmpty) ...[
                        const SizedBox(height: AppSpacing.sm),
                        Wrap(
                          spacing: AppSpacing.sm,
                          runSpacing: AppSpacing.sm,
                          children: graph.edges
                              .map(
                                (edge) => Chip(
                                  label: Text(
                                    '${labels[edge.from] ?? edge.from} ↔ ${labels[edge.to] ?? edge.to}',
                                  ),
                                  visualDensity: VisualDensity.compact,
                                ),
                              )
                              .toList(),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              ...items.map(
                (item) => Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.md),
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Expanded(
                                child: Text(
                                  item.term,
                                  style: Theme.of(
                                    context,
                                  ).textTheme.titleMedium,
                                ),
                              ),
                              Text(
                                item.level,
                                style: Theme.of(context).textTheme.labelMedium,
                              ),
                            ],
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            item.definition,
                            style: Theme.of(context).textTheme.bodyLarge,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            item.example,
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                          if (item.relatedTerms.isNotEmpty) ...[
                            const SizedBox(height: AppSpacing.sm),
                            Text(
                              'Related: ${item.relatedTerms.join(', ')}',
                              style: Theme.of(context).textTheme.labelMedium,
                            ),
                          ],
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    ),
  );
}
