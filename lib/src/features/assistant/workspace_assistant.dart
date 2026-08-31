part of '../workspace/workspace.dart';

/// Narrow recorder boundary for the canonical workspace voice composer.
///
/// Keeping the plugin behind this interface makes permission, stream failure,
/// stop timeout and unmount behavior testable without a real microphone.
abstract interface class WorkspaceAudioRecorder {
  Future<bool> hasPermission();

  Future<Stream<Uint8List>> startStream(RecordConfig config);

  Future<void> stop();

  Future<void> dispose();
}

typedef WorkspaceAudioRecorderFactory = WorkspaceAudioRecorder Function();

class PluginWorkspaceAudioRecorder implements WorkspaceAudioRecorder {
  final AudioRecorder _recorder = AudioRecorder();

  @override
  Future<bool> hasPermission() => _recorder.hasPermission();

  @override
  Future<Stream<Uint8List>> startStream(RecordConfig config) =>
      _recorder.startStream(config);

  @override
  Future<void> stop() => _recorder.stop();

  @override
  Future<void> dispose() => _recorder.dispose();
}

class _AssistantChat extends StatefulWidget {
  const _AssistantChat({
    required this.controller,
    required this.messageController,
    required this.onSubmit,
    this.speechController,
    this.recorderFactory,
    this.voiceStartupTimeout = const Duration(seconds: 10),
    this.voiceStopTimeout = const Duration(seconds: 10),
  });

  final WorkspaceController controller;
  final TextEditingController messageController;
  final Future<void> Function([String? value]) onSubmit;
  final AppController? speechController;
  final WorkspaceAudioRecorderFactory? recorderFactory;
  final Duration voiceStartupTimeout;
  final Duration voiceStopTimeout;

  @override
  State<_AssistantChat> createState() => _AssistantChatState();
}

class _AssistantChatState extends State<_AssistantChat> {
  AudioPlayer? _player;
  bool _playing = false;

  @override
  void dispose() {
    _player?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final voiceEnabled = widget.speechController != null;
    return AppSurface(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.xl,
        AppSpacing.xl,
        AppSpacing.lg,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const AppIconBadge(icon: Icons.auto_awesome, size: 32),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      voiceEnabled
                          ? 'Text + voice assistant'
                          : 'Text-first assistant',
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      voiceEnabled
                          ? 'Edit the transcript before sending it.'
                          : 'Grounded in the workspace context.',
                    ),
                  ],
                ),
              ),
              StatusPill(
                label: voiceEnabled ? 'Voice ready' : 'Ready',
                color: AppColors.success,
                icon: voiceEnabled
                    ? Icons.mic_none_outlined
                    : Icons.check_circle_outline,
              ),
            ],
          ),
          if (widget.controller.assistantContext case final assistantContext?
              when !assistantContext.isEmpty) ...[
            const SizedBox(height: AppSpacing.md),
            StatusPill(
              label: 'Focused context · ${assistantContext.label}',
              color: AppColors.accent,
              icon: Icons.push_pin_outlined,
            ),
          ],
          if (widget.controller.hasWorkspaceApi &&
              (widget.controller.conversationHistory.isNotEmpty ||
                  widget.controller.conversation.isNotEmpty)) ...[
            const SizedBox(height: AppSpacing.lg),
            _ConversationHistory(controller: widget.controller),
          ],
          if (widget.controller.conversation.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.xl),
            ...widget.controller.conversation.map(
              (turn) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: AssistantTurnBubble(
                  turn: turn,
                  onSpeak: voiceEnabled && turn.isAssistant ? _speak : null,
                  speaking: _playing,
                ),
              ),
            ),
          ],
          if (widget.controller.sending) ...[
            const LinearProgressIndicator(minHeight: 3),
            const SizedBox(height: AppSpacing.md),
            const Text('Checking the connected source...'),
            const SizedBox(height: AppSpacing.md),
          ],
          if (widget.controller.error != null) ...[
            AppSurface(
              color: AppColors.errorSoft,
              borderColor: AppColors.error.withValues(alpha: 0.25),
              padding: const EdgeInsets.all(AppSpacing.md),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(Icons.error_outline, color: AppColors.error),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      widget.controller.error!,
                      style: Theme.of(
                        context,
                      ).textTheme.bodyMedium?.copyWith(color: AppColors.error),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
          ],
          TextField(
            key: const ValueKey('assistant-composer'),
            controller: widget.messageController,
            minLines: 1,
            maxLines: 4,
            textInputAction: TextInputAction.send,
            onSubmitted: (value) => widget.onSubmit(value),
            decoration: InputDecoration(
              hintText: 'Ask about a task, decision or source...',
              suffixIcon: IconButton(
                tooltip: 'Send message',
                onPressed: widget.controller.sending
                    ? null
                    : () => widget.onSubmit(),
                icon: const Icon(Icons.arrow_upward_rounded),
              ),
            ),
          ),
          if (voiceEnabled) ...[
            const SizedBox(height: AppSpacing.md),
            _WorkspaceVoiceComposer(
              key: const ValueKey('workspace-voice-composer'),
              controller: widget.controller,
              speechController: widget.speechController!,
              messageController: widget.messageController,
              recorderFactory: widget.recorderFactory,
              startupTimeout: widget.voiceStartupTimeout,
              stopTimeout: widget.voiceStopTimeout,
            ),
          ],
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: [
              _PromptChip(
                label: 'What should I do next?',
                onPressed: () => widget.onSubmit('What should I do next?'),
              ),
              _PromptChip(
                label: 'Show current tasks',
                onPressed: () => widget.onSubmit('Show my current tasks'),
              ),
              _PromptChip(
                label: 'What is unknown?',
                onPressed: () => widget.onSubmit('What is unknown?'),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _speak(String text) async {
    if (_playing || text.trim().isEmpty) return;
    final bytes = await widget.speechController?.synthesize(text);
    if (!mounted || bytes == null || bytes.isEmpty) return;
    final player = _player ??= AudioPlayer();
    try {
      setState(() => _playing = true);
      await player.play(BytesSource(Uint8List.fromList(bytes)));
      await player.onPlayerComplete.first;
    } catch (_) {
      // The assistant remains usable as text when optional TTS is unavailable.
    } finally {
      if (mounted) setState(() => _playing = false);
    }
  }
}

class _ConversationHistory extends StatelessWidget {
  const _ConversationHistory({required this.controller});

  final WorkspaceController controller;

  @override
  Widget build(BuildContext context) {
    final history = controller.conversationHistory;
    return AppSurface(
      key: const ValueKey('assistant-conversation-history'),
      color: AppColors.surfaceMuted,
      padding: const EdgeInsets.all(AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(
                Icons.history,
                size: 18,
                color: AppColors.textSecondary,
              ),
              const SizedBox(width: AppSpacing.sm),
              const Expanded(
                child: Text(
                  'Recent conversations',
                  style: TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
              TextButton.icon(
                onPressed: controller.sending || controller.conversationLoading
                    ? null
                    : controller.startNewConversation,
                icon: const Icon(Icons.add, size: 18),
                label: const Text('New'),
              ),
            ],
          ),
          if (controller.conversationLoading) ...[
            const SizedBox(height: AppSpacing.xs),
            const LinearProgressIndicator(minHeight: 2),
          ],
          if (history.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.xs),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                for (final item in history)
                  ActionChip(
                    avatar: Icon(
                      item.id == controller.conversationId
                          ? Icons.check_circle_outline
                          : Icons.chat_bubble_outline,
                      size: 16,
                    ),
                    label: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 240),
                      child: Text(
                        item.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    onPressed:
                        controller.conversationLoading || controller.sending
                        ? null
                        : () => controller.openConversation(item.id),
                  ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _WorkspaceVoiceComposer extends StatefulWidget {
  const _WorkspaceVoiceComposer({
    super.key,
    required this.controller,
    required this.speechController,
    required this.messageController,
    this.recorderFactory,
    this.startupTimeout = const Duration(seconds: 10),
    this.stopTimeout = const Duration(seconds: 10),
  });

  final WorkspaceController controller;
  final AppController speechController;
  final TextEditingController messageController;
  final WorkspaceAudioRecorderFactory? recorderFactory;
  final Duration startupTimeout;
  final Duration stopTimeout;

  @override
  State<_WorkspaceVoiceComposer> createState() =>
      _WorkspaceVoiceComposerState();
}

class _WorkspaceVoiceComposerState extends State<_WorkspaceVoiceComposer> {
  WorkspaceAudioRecorder? _recorder;
  StreamSubscription<Uint8List>? _recordingSubscription;
  final List<int> _audio = <int>[];
  bool _recording = false;
  bool _starting = false;
  bool _busy = false;
  bool _streamFailed = false;
  int _recordingGeneration = 0;
  String _state = 'Hold to talk. The transcript will stay editable.';

  @override
  void dispose() {
    _recordingGeneration++;
    _recordingSubscription?.cancel();
    _recorder?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      HoldToTalkButton(
        enabled: !_busy,
        recording: _recording,
        onPressStart: _startRecording,
        onPressEnd: _finishRecording,
        onPressCancel: _cancelRecording,
      ),
      const SizedBox(height: AppSpacing.xs),
      Text(
        _state,
        key: const ValueKey('workspace-voice-state'),
        style: Theme.of(context).textTheme.bodySmall,
      ),
    ],
  );

  WorkspaceAudioRecorder get _audioRecorder => _recorder ??=
      widget.recorderFactory?.call() ?? PluginWorkspaceAudioRecorder();

  Future<void> _startRecording() async {
    if (_recording || _busy) return;
    final generation = ++_recordingGeneration;
    _starting = true;
    _streamFailed = false;
    setState(() => _state = 'Requesting microphone permission...');
    bool allowed;
    try {
      allowed = await _audioRecorder.hasPermission().timeout(
        widget.startupTimeout,
      );
    } catch (_) {
      _starting = false;
      if (mounted && generation == _recordingGeneration) {
        setState(() => _state = 'Microphone permission failed.');
      }
      return;
    }
    if (!mounted || generation != _recordingGeneration) return;
    if (!allowed) {
      _starting = false;
      if (mounted) setState(() => _state = 'Microphone permission denied.');
      return;
    }

    _audio.clear();
    try {
      final stream = await _audioRecorder
          .startStream(
            const RecordConfig(
              encoder: AudioEncoder.pcm16bits,
              sampleRate: 16000,
              numChannels: 1,
              androidConfig: AndroidRecordConfig(
                manageBluetooth: false,
                audioSource: AndroidAudioSource.voiceRecognition,
              ),
            ),
          )
          .timeout(widget.startupTimeout);
      if (!mounted || generation != _recordingGeneration) {
        _starting = false;
        unawaited(_audioRecorder.stop());
        return;
      }
      _recordingSubscription = stream.listen(
        _audio.addAll,
        onError: (_) {
          if (!mounted || generation != _recordingGeneration) return;
          _starting = false;
          _streamFailed = true;
          _recording = false;
          _recordingSubscription = null;
          // Defer the stop until the stream has finished dispatching its
          // error event; some recorder implementations close the stream in
          // stop(), which must not happen re-entrantly from onError.
          unawaited(Future<void>.microtask(_audioRecorder.stop));
          setState(
            () => _state = 'Microphone stream failed. Recording cancelled.',
          );
        },
      );
      if (mounted && generation == _recordingGeneration) {
        _starting = false;
        setState(() {
          _recording = true;
          _state = 'Listening... release to transcribe.';
        });
      }
    } catch (_) {
      _starting = false;
      if (mounted && generation == _recordingGeneration) {
        setState(() => _state = 'Could not start microphone.');
      }
    }
  }

  Future<void> _finishRecording() async {
    if (_busy || _streamFailed) return;
    if (!_recording) {
      if (_starting) _cancelRecording();
      return;
    }
    final generation = _recordingGeneration;
    _starting = false;
    setState(() {
      _recording = false;
      _busy = true;
      _state = 'Transcribing...';
    });
    try {
      try {
        await _audioRecorder.stop().timeout(widget.stopTimeout);
      } on TimeoutException {
        _recordingGeneration++;
        final subscription = _recordingSubscription;
        _recordingSubscription = null;
        unawaited(_cancelRecordingSubscription(subscription));
        if (mounted) {
          setState(() => _state = 'Recording could not stop. Try again.');
        }
        return;
      }
      await Future<void>.delayed(const Duration(milliseconds: 150));
      final subscription = _recordingSubscription;
      _recordingSubscription = null;
      unawaited(_cancelRecordingSubscription(subscription));
      if (!mounted || generation != _recordingGeneration || _streamFailed) {
        return;
      }
      final audio = List<int>.from(_audio);
      if (audio.isEmpty) {
        if (mounted) setState(() => _state = 'No audio captured. Try again.');
        return;
      }
      await widget.speechController.transcribeAudio(
        _wrapWorkspacePcm16AsWav(audio),
        'audio/wav',
      );
      if (!mounted) return;
      if (widget.speechController.error != null) {
        setState(() => _state = 'Transcription failed. Try again.');
        return;
      }
      final transcript = widget.speechController.speakingSession?.transcript;
      if (transcript != null && transcript.trim().isNotEmpty) {
        widget.messageController
          ..text = transcript.trim()
          ..selection = TextSelection.collapsed(
            offset: transcript.trim().length,
          );
        setState(() => _state = 'Transcript ready — edit it before sending.');
      } else {
        setState(() => _state = 'No transcript returned. Try again.');
      }
    } catch (_) {
      if (mounted) setState(() => _state = 'Transcription failed. Try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _cancelRecording() {
    if (_busy || (!_recording && !_starting)) return;
    _recordingGeneration++;
    _starting = false;
    setState(() {
      _recording = false;
      _streamFailed = false;
      _state = 'Recording cancelled.';
    });
    unawaited(_audioRecorder.stop());
    final subscription = _recordingSubscription;
    _recordingSubscription = null;
    unawaited(_cancelRecordingSubscription(subscription));
    _audio.clear();
  }

  Future<void> _cancelRecordingSubscription(
    StreamSubscription<Uint8List>? subscription,
  ) async {
    try {
      await subscription?.cancel();
    } catch (_) {
      // Recorder shutdown is best effort after the transcript boundary has
      // been detached from the stream.
    }
  }
}

Uint8List _wrapWorkspacePcm16AsWav(
  List<int> pcmBytes, {
  int sampleRate = 16000,
}) {
  final pcm = Uint8List.fromList(pcmBytes);
  final wav = ByteData(44 + pcm.length);
  _writeWorkspaceAscii(wav, 0, 'RIFF');
  wav.setUint32(4, 36 + pcm.length, Endian.little);
  _writeWorkspaceAscii(wav, 8, 'WAVE');
  _writeWorkspaceAscii(wav, 12, 'fmt ');
  wav.setUint32(16, 16, Endian.little);
  wav.setUint16(20, 1, Endian.little);
  wav.setUint16(22, 1, Endian.little);
  wav.setUint32(24, sampleRate, Endian.little);
  wav.setUint32(28, sampleRate * 2, Endian.little);
  wav.setUint16(32, 2, Endian.little);
  wav.setUint16(34, 16, Endian.little);
  _writeWorkspaceAscii(wav, 36, 'data');
  wav.setUint32(40, pcm.length, Endian.little);
  final result = wav.buffer.asUint8List();
  result.setRange(44, result.length, pcm);
  return result;
}

void _writeWorkspaceAscii(ByteData data, int offset, String value) {
  for (var index = 0; index < value.length; index++) {
    data.setUint8(offset + index, value.codeUnitAt(index));
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
  const AssistantTurnBubble({
    super.key,
    required this.turn,
    this.onSpeak,
    this.speaking = false,
  });

  final AssistantTurn turn;
  final Future<void> Function(String text)? onSpeak;
  final bool speaking;

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
                  if (isAssistant && onSpeak != null) ...[
                    const SizedBox(width: AppSpacing.sm),
                    IconButton(
                      visualDensity: VisualDensity.compact,
                      tooltip: speaking ? 'Playing audio' : 'Read aloud',
                      onPressed: speaking ? null : () => onSpeak!(turn.content),
                      icon: Icon(
                        speaking
                            ? Icons.volume_up_outlined
                            : Icons.volume_down_outlined,
                        size: 18,
                      ),
                    ),
                  ],
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
              if (turn.actionDetails.isNotEmpty) ...[
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Suggested actions · preview only',
                  style: Theme.of(context).textTheme.labelMedium,
                ),
                const SizedBox(height: AppSpacing.sm),
                ...turn.actionDetails.map(
                  (action) => _SuggestedActionCard(action: action),
                ),
              ] else if (turn.suggestedActions.isNotEmpty) ...[
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
              if (turn.receiptDetails.isNotEmpty) ...[
                const SizedBox(height: AppSpacing.md),
                Text(
                  'Action receipts',
                  style: Theme.of(context).textTheme.labelMedium,
                ),
                const SizedBox(height: AppSpacing.sm),
                ...turn.receiptDetails.map(
                  (receipt) => _AssistantReceiptLine(receipt: receipt),
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

class _SuggestedActionCard extends StatelessWidget {
  const _SuggestedActionCard({required this.action});

  final AssistantSuggestedAction action;

  @override
  Widget build(BuildContext context) => AppSurface(
    margin: const EdgeInsets.only(bottom: AppSpacing.sm),
    padding: const EdgeInsets.all(AppSpacing.md),
    color: AppColors.warningSoft,
    borderColor: AppColors.warning.withValues(alpha: 0.35),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Icon(
              Icons.shield_outlined,
              size: 18,
              color: AppColors.warning,
            ),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Text(
                action.label,
                style: Theme.of(context).textTheme.labelLarge,
              ),
            ),
            const StatusPill(label: 'Preview only', color: AppColors.warning),
          ],
        ),
        if (action.target.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.xs),
          Text(action.target),
        ],
        const SizedBox(height: AppSpacing.xs),
        Text(
          action.requiresConfirmation
              ? 'Requires explicit confirmation before execution.'
              : 'This suggestion cannot execute by itself.',
          style: Theme.of(context).textTheme.bodyMedium,
        ),
      ],
    ),
  );
}

class _AssistantReceiptLine extends StatelessWidget {
  const _AssistantReceiptLine({required this.receipt});

  final AssistantActionReceipt receipt;

  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      const Icon(Icons.verified_outlined, size: 18, color: AppColors.success),
      const SizedBox(width: AppSpacing.sm),
      Expanded(
        child: Text('${receipt.operation} · ${receipt.status} · ${receipt.id}'),
      ),
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
    final provenanceColor = citation.origin.isPreview
        ? AppColors.accent
        : citation.stale
        ? AppColors.warning
        : AppColors.success;
    return AppSurface(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      padding: const EdgeInsets.all(AppSpacing.md),
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
              StatusPill(
                label: provenanceLabel,
                color: provenanceColor,
                icon: citation.stale
                    ? Icons.schedule_outlined
                    : citation.origin.isPreview
                    ? Icons.visibility_outlined
                    : Icons.verified_outlined,
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
