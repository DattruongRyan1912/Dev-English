import 'dart:typed_data';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';

import '../app_controller.dart';
import '../models.dart';
import '../theme.dart';

class RoleplayScreen extends StatefulWidget {
  const RoleplayScreen({
    super.key,
    required this.controller,
    this.scenarioType,
  });

  final AppController controller;
  final String? scenarioType;

  @override
  State<RoleplayScreen> createState() => _RoleplayScreenState();
}

class _RoleplayScreenState extends State<RoleplayScreen> {
  final _answerController = TextEditingController();
  final _scrollController = ScrollController();
  AudioPlayer? _player;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_scrollToLatest);
    widget.controller.loadRoleplay();
  }

  @override
  void dispose() {
    widget.controller.removeListener(_scrollToLatest);
    _answerController.dispose();
    _scrollController.dispose();
    _player?.dispose();
    super.dispose();
  }

  void _scrollToLatest() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOut,
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        title: const Text('AI Roleplay'),
        backgroundColor: AppColors.background,
        surfaceTintColor: Colors.transparent,
      ),
      body: SafeArea(
        child: AnimatedBuilder(
          animation: widget.controller,
          builder: (context, _) {
            if (widget.controller.conversation != null) {
              return _ConversationView(
                controller: widget.controller,
                answerController: _answerController,
                scrollController: _scrollController,
                onSend: _send,
                onSpeak: _speak,
              );
            }
            if (widget.controller.roleplayLoading) {
              return const Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    CircularProgressIndicator(),
                    SizedBox(height: AppSpacing.lg),
                    Text('Loading roleplay scenarios...'),
                  ],
                ),
              );
            }
            if (widget.controller.scenarios.isEmpty &&
                widget.controller.roleplayError != null) {
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text(
                        'Roleplay is temporarily unavailable.',
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        widget.controller.roleplayError!,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      FilledButton.icon(
                        onPressed: widget.controller.loadRoleplay,
                        icon: const Icon(Icons.refresh),
                        label: const Text('Retry roleplay'),
                      ),
                    ],
                  ),
                ),
              );
            }
            return _ScenarioPicker(
              controller: widget.controller,
              onStart: _start,
              scenarioType: widget.scenarioType,
            );
          },
        ),
      ),
    );
  }

  Future<void> _start(RoleplayScenario scenario) async {
    await widget.controller.startRoleplay(scenario.id);
  }

  Future<void> _send() async {
    final answer = _answerController.text.trim();
    if (answer.isEmpty) return;
    _answerController.clear();
    await widget.controller.sendRoleplayTurn(answer);
  }

  Future<void> _speak(String text) async {
    final bytes = await widget.controller.synthesize(text);
    if (bytes != null && mounted) {
      final player = _player ??= AudioPlayer();
      try {
        await player.play(BytesSource(Uint8List.fromList(bytes)));
      } catch (_) {
        // The backend result remains usable when the current web shell has no
        // registered audio plugin; native/web builds can still opt into TTS.
      }
    }
  }
}

class _ScenarioPicker extends StatelessWidget {
  const _ScenarioPicker({
    required this.controller,
    required this.onStart,
    this.scenarioType,
  });

  final AppController controller;
  final ValueChanged<RoleplayScenario> onStart;
  final String? scenarioType;

  @override
  Widget build(BuildContext context) {
    final scenarios = scenarioType == null
        ? controller.scenarios
        : controller.scenarios
              .where((scenario) => scenario.type == scenarioType)
              .toList();
    if (scenarios.isEmpty) {
      return Center(
        child: Text(
          'No scenarios are available for this practice yet.',
          style: Theme.of(context).textTheme.bodyLarge,
        ),
      );
    }
    return ListView(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.md,
        AppSpacing.xl,
        AppSpacing.section,
      ),
      children: [
        Text(
          'Choose one work conversation.',
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: AppSpacing.sm),
        Text(
          'The partner asks one focused follow-up at a time.',
          style: Theme.of(context).textTheme.bodyMedium,
        ),
        const SizedBox(height: AppSpacing.xl),
        ...scenarios.map(
          (scenario) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.md),
            child: Card(
              child: ListTile(
                contentPadding: const EdgeInsets.all(AppSpacing.lg),
                title: Text(
                  scenario.title,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                subtitle: Padding(
                  padding: const EdgeInsets.only(top: AppSpacing.sm),
                  child: Text(
                    '${scenario.partnerRole} · ${scenario.level}\n${scenario.goal}',
                  ),
                ),
                isThreeLine: true,
                trailing: const Icon(Icons.chevron_right),
                onTap: () => onStart(scenario),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _ConversationView extends StatelessWidget {
  const _ConversationView({
    required this.controller,
    required this.answerController,
    required this.scrollController,
    required this.onSend,
    required this.onSpeak,
  });

  final AppController controller;
  final TextEditingController answerController;
  final ScrollController scrollController;
  final VoidCallback onSend;
  final ValueChanged<String> onSpeak;

  @override
  Widget build(BuildContext context) {
    final conversation = controller.conversation!;
    final pendingReply = controller.working;
    return Column(
      children: [
        Expanded(
          child: ListView.builder(
            controller: scrollController,
            padding: const EdgeInsets.all(AppSpacing.xl),
            itemCount: conversation.messages.length + (pendingReply ? 1 : 0),
            itemBuilder: (context, index) {
              if (pendingReply && index == conversation.messages.length) {
                return Align(
                  alignment: Alignment.centerLeft,
                  child: Container(
                    constraints: const BoxConstraints(maxWidth: 560),
                    margin: const EdgeInsets.only(bottom: AppSpacing.md),
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    decoration: BoxDecoration(
                      color: AppColors.surface,
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: AppColors.border),
                    ),
                    child: Text(
                      'Partner is thinking...',
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  ),
                );
              }
              final message = conversation.messages[index];
              final fromUser = message.role == 'user';
              return Align(
                alignment: fromUser
                    ? Alignment.centerRight
                    : Alignment.centerLeft,
                child: Container(
                  constraints: const BoxConstraints(maxWidth: 560),
                  margin: const EdgeInsets.only(bottom: AppSpacing.md),
                  padding: const EdgeInsets.all(AppSpacing.lg),
                  decoration: BoxDecoration(
                    color: fromUser ? AppColors.accentSoft : AppColors.surface,
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(color: AppColors.border),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Flexible(
                        child: Text(
                          message.content,
                          style: Theme.of(context).textTheme.bodyLarge,
                        ),
                      ),
                      if (!fromUser)
                        IconButton(
                          onPressed: () => onSpeak(message.content),
                          tooltip: 'Read aloud',
                          icon: const Icon(Icons.volume_up_outlined),
                        ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        if (controller.lastRoleplayTurn != null)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
            child: Text(
              'Feedback: ${controller.lastRoleplayTurn!.feedback.nextAction}',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
        Padding(
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.md,
            AppSpacing.xl,
            AppSpacing.xl,
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                child: TextField(
                  controller: answerController,
                  minLines: 1,
                  maxLines: 4,
                  enabled: !controller.working,
                  textInputAction: TextInputAction.send,
                  onSubmitted: (_) {
                    if (!controller.working) onSend();
                  },
                  decoration: const InputDecoration(
                    hintText: 'Reply as a developer...',
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              IconButton.filled(
                onPressed: controller.working ? null : onSend,
                tooltip: 'Send reply',
                icon: const Icon(Icons.send),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
