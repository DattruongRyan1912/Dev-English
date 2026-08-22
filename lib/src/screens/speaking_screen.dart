import 'dart:async';
import 'dart:typed_data';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
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
  AudioPlayer? _player;
  final List<int> _audio = <int>[];
  bool _recording = false;
  bool _playing = false;
  String _state = 'Ready';

  @override
  void dispose() {
    _recordingSubscription?.cancel();
    _player?.dispose();
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
                    canAssess: _audio.isNotEmpty,
                    onPlay: _playFeedback,
                    playing: _playing,
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
        _state = 'Uploading';
      });
      try {
        await _recorder.stop().timeout(const Duration(seconds: 10));
        await _recordingSubscription?.cancel();
        _recordingSubscription = null;
      } catch (_) {
        if (mounted) {
          setState(() => _state = 'Provider/network failure — Retry');
        }
        return;
      }
      if (_audio.isEmpty) {
        setState(() => _state = 'Recording failed — Retry');
        return;
      }
      if (mounted) setState(() => _state = 'Transcribing');
      await widget.controller.transcribeAudio(
        _audioForUpload(),
        _audioMimeType,
      );
      if (!mounted) return;
      setState(
        () => _state = widget.controller.error == null
            ? 'Success — transcript ready'
            : 'Provider/network failure — Retry',
      );
      final transcript = widget.controller.speakingSession?.transcript;
      if (transcript != null && transcript.isNotEmpty) {
        _transcriptController.text = transcript;
      }
      return;
    }
    setState(() => _state = 'Requesting microphone permission');
    bool allowed;
    try {
      allowed = await _recorder.hasPermission().timeout(
        const Duration(seconds: 10),
      );
    } catch (_) {
      if (mounted) setState(() => _state = 'Permission request failed — Retry');
      return;
    }
    if (!allowed) {
      setState(() => _state = 'Permission denied — Retry');
      return;
    }
    _audio.clear();
    try {
      final stream = await _recorder
          .startStream(
            const RecordConfig(
              encoder: kIsWeb ? AudioEncoder.pcm16bits : AudioEncoder.opus,
              sampleRate: 16000,
              numChannels: 1,
              echoCancel: true,
              noiseSuppress: true,
            ),
          )
          .timeout(const Duration(seconds: 10));
      _recordingSubscription = stream.listen(_audio.addAll);
    } catch (_) {
      if (mounted) setState(() => _state = 'Provider/network failure — Retry');
      return;
    }
    if (mounted) {
      setState(() {
        _recording = true;
        _state = 'Recording';
      });
    }
  }

  Future<void> _saveManualTranscript() async {
    if (_transcriptController.text.trim().isEmpty) return;
    setState(() => _state = 'Saving transcript');
    await widget.controller.saveTranscript(_transcriptController.text);
    if (mounted) {
      setState(
        () => _state = widget.controller.error == null
            ? 'Success — transcript saved'
            : 'Provider/network failure — Retry',
      );
    }
  }

  Future<void> _assess() async {
    if (_audio.isEmpty) return;
    setState(() => _state = 'Assessing pronunciation');
    await widget.controller.assessSpeaking(
      _audioForUpload(),
      _audioMimeType,
      _transcriptController.text,
    );
    if (mounted) {
      setState(
        () => _state = widget.controller.error == null
            ? 'Success — pronunciation assessed'
            : 'Provider/network failure — Retry',
      );
    }
  }

  Future<void> _playFeedback() async {
    final session = widget.controller.speakingSession;
    if (session == null || _playing) return;
    setState(() => _state = 'Synthesizing feedback');
    final pronunciation = session.pronunciation;
    final text = pronunciation == null
        ? session.transcript
        : 'Your pronunciation score is ${pronunciation.score.round()} percent. '
              'Accuracy ${pronunciation.accuracy.round()} percent. '
              'Fluency ${pronunciation.fluency.round()} percent. '
              'Completeness ${pronunciation.completeness.round()} percent. '
              'Prosody ${pronunciation.prosody.round()} percent.';
    final bytes = await widget.controller.synthesize(text);
    if (!mounted) return;
    if (bytes == null || bytes.isEmpty) {
      setState(() => _state = 'Provider/network failure — Retry');
      return;
    }
    final player = _player ??= AudioPlayer();
    try {
      setState(() {
        _playing = true;
        _state = 'Playing feedback';
      });
      await player.play(BytesSource(Uint8List.fromList(bytes)));
      await player.onPlayerComplete.first;
      if (mounted) setState(() => _state = 'Success — feedback played');
    } catch (_) {
      if (mounted) setState(() => _state = 'Provider/network failure — Retry');
    } finally {
      if (mounted) setState(() => _playing = false);
    }
  }

  String get _audioMimeType => kIsWeb ? 'audio/wav' : 'audio/webm';

  List<int> _audioForUpload() {
    if (!kIsWeb) return List<int>.from(_audio);

    final pcm = Uint8List.fromList(_audio);
    final wav = ByteData(44 + pcm.length);
    _writeAscii(wav, 0, 'RIFF');
    wav.setUint32(4, 36 + pcm.length, Endian.little);
    _writeAscii(wav, 8, 'WAVE');
    _writeAscii(wav, 12, 'fmt ');
    wav.setUint32(16, 16, Endian.little);
    wav.setUint16(20, 1, Endian.little);
    wav.setUint16(22, 1, Endian.little);
    wav.setUint32(24, 16000, Endian.little);
    wav.setUint32(28, 32000, Endian.little);
    wav.setUint16(32, 2, Endian.little);
    wav.setUint16(34, 16, Endian.little);
    _writeAscii(wav, 36, 'data');
    wav.setUint32(40, pcm.length, Endian.little);
    final result = wav.buffer.asUint8List();
    result.setRange(44, result.length, pcm);
    return result;
  }
}

void _writeAscii(ByteData data, int offset, String value) {
  for (var index = 0; index < value.length; index++) {
    data.setUint8(offset + index, value.codeUnitAt(index));
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
  const _SessionResult({
    required this.session,
    required this.onAssess,
    required this.canAssess,
    required this.onPlay,
    required this.playing,
  });

  final SpeakingSession session;
  final VoidCallback onAssess;
  final bool canAssess;
  final VoidCallback onPlay;
  final bool playing;

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
            OutlinedButton.icon(
              onPressed: playing ? null : onPlay,
              icon: Icon(playing ? Icons.volume_up : Icons.play_arrow),
              label: Text(playing ? 'Playing feedback…' : 'Play feedback'),
            ),
            const SizedBox(height: AppSpacing.md),
            if (pronunciation == null)
              OutlinedButton.icon(
                onPressed: canAssess ? onAssess : null,
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
