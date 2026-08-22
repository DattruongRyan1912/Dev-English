import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../theme.dart';

class WorkImportScreen extends StatefulWidget {
  const WorkImportScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<WorkImportScreen> createState() => _WorkImportScreenState();
}

class _WorkImportScreenState extends State<WorkImportScreen> {
  final _githubUrlController = TextEditingController();
  final _titleController = TextEditingController();
  final _contentController = TextEditingController();
  String _sourceType = 'error';

  @override
  void dispose() {
    _githubUrlController.dispose();
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        title: const Text('Learn From My Work'),
        backgroundColor: AppColors.background,
        surfaceTintColor: Colors.transparent,
      ),
      body: SafeArea(
        child: AnimatedBuilder(
          animation: widget.controller,
          builder: (context, _) => SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.xl,
              AppSpacing.md,
              AppSpacing.xl,
              AppSpacing.section,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Turn real work into one useful lesson.',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  'Paste an error, issue, PR, code excerpt or documentation. Do not include secrets or credentials.',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: AppSpacing.xl),
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Import from GitHub',
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          'Use a public repository, README, issue or pull request URL. Private access stays on the backend token.',
                          style: Theme.of(context).textTheme.bodyMedium,
                        ),
                        const SizedBox(height: AppSpacing.md),
                        TextField(
                          controller: _githubUrlController,
                          keyboardType: TextInputType.url,
                          decoration: const InputDecoration(
                            labelText: 'GitHub URL',
                            hintText: 'https://github.com/org/repo/pull/42',
                          ),
                        ),
                        const SizedBox(height: AppSpacing.md),
                        SizedBox(
                          width: double.infinity,
                          child: OutlinedButton.icon(
                            onPressed: widget.controller.importing
                                ? null
                                : _importGitHub,
                            icon: const Icon(Icons.code),
                            label: const Text('Import GitHub source'),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.section),
                DropdownButtonFormField<String>(
                  initialValue: _sourceType,
                  decoration: const InputDecoration(labelText: 'Source type'),
                  items: const [
                    DropdownMenuItem(
                      value: 'error',
                      child: Text('Error / log'),
                    ),
                    DropdownMenuItem(
                      value: 'issue',
                      child: Text('Issue / bug report'),
                    ),
                    DropdownMenuItem(value: 'code', child: Text('Code')),
                    DropdownMenuItem(
                      value: 'documentation',
                      child: Text('Documentation'),
                    ),
                  ],
                  onChanged: (value) =>
                      setState(() => _sourceType = value ?? 'error'),
                ),
                const SizedBox(height: AppSpacing.lg),
                TextField(
                  controller: _titleController,
                  decoration: const InputDecoration(
                    labelText: 'Short title',
                    hintText: 'e.g. Upload API returns 500',
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                TextField(
                  controller: _contentController,
                  minLines: 10,
                  maxLines: 16,
                  textCapitalization: TextCapitalization.sentences,
                  decoration: const InputDecoration(
                    labelText: 'Work context',
                    alignLabelWithHint: true,
                    hintText: 'Paste only the context needed for the lesson...',
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton(
                    onPressed: widget.controller.importing ? null : _submit,
                    child: Text(
                      widget.controller.importing
                          ? 'Analyzing your work…'
                          : 'Analyze work',
                    ),
                  ),
                ),
                if (widget.controller.lastWorkImport != null) ...[
                  const SizedBox(height: AppSpacing.section),
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(AppSpacing.xl),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Suggested mission',
                            style: Theme.of(context).textTheme.titleLarge,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            'Domain: ${widget.controller.lastWorkImport!.domain}',
                            style: Theme.of(context).textTheme.bodyMedium,
                          ),
                          if (widget
                              .controller
                              .lastWorkImport!
                              .sourceUrl
                              .isNotEmpty) ...[
                            const SizedBox(height: AppSpacing.xs),
                            Text(
                              widget.controller.lastWorkImport!.sourceUrl,
                              style: Theme.of(context).textTheme.labelMedium,
                            ),
                          ],
                          const SizedBox(height: AppSpacing.lg),
                          Text(
                            widget.controller.lastWorkImport!.mission.title,
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            widget.controller.lastWorkImport!.mission.prompt,
                            style: Theme.of(context).textTheme.bodyLarge,
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          Wrap(
                            spacing: AppSpacing.sm,
                            runSpacing: AppSpacing.sm,
                            children: widget.controller.lastWorkImport!.terms
                                .map(
                                  (term) => Chip(
                                    label: Text(term),
                                    visualDensity: VisualDensity.compact,
                                  ),
                                )
                                .toList(),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
                if (widget.controller.error != null) ...[
                  const SizedBox(height: AppSpacing.lg),
                  Text(
                    widget.controller.error!,
                    style: Theme.of(
                      context,
                    ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _submit() {
    widget.controller.importWork(
      sourceType: _sourceType,
      title: _titleController.text,
      content: _contentController.text,
    );
  }

  void _importGitHub() {
    final url = _githubUrlController.text.trim();
    if (url.isEmpty) return;
    widget.controller.importGitHub(url);
  }
}
