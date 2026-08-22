import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:record/record.dart';

import '../app_controller.dart';
import '../models.dart';
import '../theme.dart';

class SpeakingScreen extends StatefulWidget {
  const SpeakingScreen({super.key, required this.controller});

  final AppController controller;

  @override
  State<SpeakingScreen> createState() => _SpeakingScreenState();
}

class _SpeakingScreenState extends State<SpeakingScreen> {
  final AudioRecorder _recorder = AudioRecorder();
  final TextEditingController _transcriptController = TextEditingController();
  StreamSubscription<Uint8List>? _recordingSubscription;
  final List<int> _audio = <int>[];
  bool _recording = false;
  String _state = 'Ready';

  @override
  void dispose() {
    _recordingSubscription?.cancel();
    _recorder.dispose();
    _transcriptController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      appBar: AppBar(
        title: const Text('Speaking'),
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
                  'Explain one technical decision clearly.',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  'Use the microphone for STT, then retry from the transcript. Raw audio is not retained by the app after processing.',
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
                const SizedBox(height: AppSpacing.xl),
                _StateBanner(state: _state),
                const SizedBox(height: AppSpacing.lg),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: widget.controller.working
                        ? null
                        : _toggleRecording,
                    icon: Icon(_recording ? Icons.stop : Icons.mic),
                    label: Text(
                      _recording ? 'Stop recording' : 'Start recording',
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
                TextField(
                  controller: _transcriptController,
                  minLines: 5,
                  maxLines: 10,
                  decoration: const InputDecoration(
                    labelText: 'Transcript / manual fallback',
                    alignLabelWithHint: true,
                    hintText:
                        'Paste or type what you would say to your teammate...',
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                OutlinedButton(
                  onPressed: widget.controller.working
                      ? null
                      : _saveManualTranscript,
                  child: const Text('Save transcript'),
                ),
                if (widget.controller.speakingSession != null) ...[
                  const SizedBox(height: AppSpacing.xl),
                  _SessionResult(
                    session: widget.controller.speakingSession!,
                    onAssess: _assess,
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

  Future<void> _toggleRecording() async {
    if (_recording) {
      setState(() {
        _recording = false;
        _state = 'Processing';
      });
      await _recorder.stop();
      await _recordingSubscription?.cancel();
      _recordingSubscription = null;
      if (_audio.isEmpty) {
        setState(() => _state = 'Error');
        return;
      }
      await widget.controller.transcribeAudio(_audio, 'audio/webm');
      if (!mounted) return;
      setState(() => _state = 'Result');
      final transcript = widget.controller.speakingSession?.transcript;
      if (transcript != null && transcript.isNotEmpty) {
        _transcriptController.text = transcript;
      }
      return;
    }
    final allowed = await _recorder.hasPermission();
    if (!allowed) {
      setState(() => _state = 'Error');
      return;
    }
    _audio.clear();
    final stream = await _recorder.startStream(
      const RecordConfig(
        encoder: AudioEncoder.opus,
        sampleRate: 16000,
        numChannels: 1,
        echoCancel: true,
        noiseSuppress: true,
      ),
    );
    _recordingSubscription = stream.listen(_audio.addAll);
    if (mounted) {
      setState(() {
        _recording = true;
        _state = 'Recording';
      });
    }
  }

  Future<void> _saveManualTranscript() async {
    if (_transcriptController.text.trim().isEmpty) return;
    setState(() => _state = 'Processing');
    await widget.controller.saveTranscript(_transcriptController.text);
    if (mounted) setState(() => _state = 'Result');
  }

  Future<void> _assess() async {
    if (_audio.isEmpty) return;
    setState(() => _state = 'Evaluating');
    await widget.controller.assessSpeaking(
      _audio,
      'audio/webm',
      _transcriptController.text,
    );
    if (mounted) setState(() => _state = 'Result');
  }
}

class _StateBanner extends StatelessWidget {
  const _StateBanner({required this.state});

  final String state;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.accentSoft,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: AppColors.border),
      ),
      child: Row(
        children: [
          const Icon(Icons.graphic_eq, color: AppColors.accent),
          const SizedBox(width: AppSpacing.md),
          Text(
            'Speech state: $state',
            style: Theme.of(context).textTheme.bodyLarge,
          ),
        ],
      ),
    );
  }
}

class _SessionResult extends StatelessWidget {
  const _SessionResult({required this.session, required this.onAssess});

  final SpeakingSession session;
  final VoidCallback onAssess;

  @override
  Widget build(BuildContext context) {
    final pronunciation = session.pronunciation;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Transcript', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: AppSpacing.sm),
            Text(
              session.transcript,
              style: Theme.of(context).textTheme.bodyLarge,
            ),
            const SizedBox(height: AppSpacing.lg),
            if (pronunciation == null)
              OutlinedButton.icon(
                onPressed: onAssess,
                icon: const Icon(Icons.assessment_outlined),
                label: const Text('Assess pronunciation'),
              )
            else
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Wrap(
                    spacing: AppSpacing.lg,
                    runSpacing: AppSpacing.sm,
                    children: [
                      Text('Score ${pronunciation.score.round()}%'),
                      Text('Accuracy ${pronunciation.accuracy.round()}%'),
                      Text('Fluency ${pronunciation.fluency.round()}%'),
                      Text(
                        'Completeness ${pronunciation.completeness.round()}%',
                      ),
                      Text('Prosody ${pronunciation.prosody.round()}%'),
                    ],
                  ),
                  if (pronunciation.words.isNotEmpty) ...[
                    const SizedBox(height: AppSpacing.lg),
                    Text(
                      'Word-level report',
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Wrap(
                      spacing: AppSpacing.sm,
                      runSpacing: AppSpacing.sm,
                      children: pronunciation.words.entries
                          .map(
                            (entry) => Chip(
                              label: Text(
                                '${entry.key} ${entry.value.round()}%',
                              ),
                              visualDensity: VisualDensity.compact,
                            ),
                          )
                          .toList(),
                    ),
                  ],
                ],
              ),
          ],
        ),
      ),
    );
  }
}
