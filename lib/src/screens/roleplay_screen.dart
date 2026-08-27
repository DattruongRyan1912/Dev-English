import 'dart:async';
import 'dart:typed_data';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:record/record.dart';

import '../app_controller.dart';
import '../components.dart';
import '../models.dart';
import '../theme.dart';

abstract interface class RoleplayAudioRecorder {
  Future<bool> hasPermission();

  Future<Stream<Uint8List>> startStream(RecordConfig config);

  Future<void> stop();

  Future<void> dispose();
}

class PlatformRoleplayAudioRecorder implements RoleplayAudioRecorder {
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

class RoleplayScreen extends StatefulWidget {
  const RoleplayScreen({
    super.key,
    required this.controller,
    this.scenarioType,
    this.recorder,
    this.recordingStopTimeout = const Duration(seconds: 10),
  });

  final AppController controller;
  final String? scenarioType;
  final RoleplayAudioRecorder? recorder;
  final Duration recordingStopTimeout;

  @override
  State<RoleplayScreen> createState() => _RoleplayScreenState();
}

class _RoleplayScreenState extends State<RoleplayScreen> {
  static const _recordingStartTimeout = Duration(seconds: 10);
  static const _recordingMaxDuration = Duration(seconds: 60);
  static const _recordingMaxBytes = 2 * 1024 * 1024;

  final _answerController = TextEditingController();
  final List<int> _audio = <int>[];
  StreamSubscription<Uint8List>? _recordingSubscription;
  Completer<void>? _recordingDrain;
  late final RoleplayAudioRecorder _recorder;
  Future<void>? _recorderStopFuture;
  Timer? _recordingTimer;
  AudioPlayer? _player;
  bool _recording = false;
  bool _starting = false;
  bool _finishing = false;
  bool _discarding = false;
  bool _disposed = false;
  bool _pressActive = false;
  bool _recorderActive = false;
  bool _recordingLimitReached = false;
  int _recordingSession = 0;
  String _voiceState = 'Ready';

  @override
  void initState() {
    super.initState();
    _recorder = widget.recorder ?? PlatformRoleplayAudioRecorder();
    widget.controller.loadRoleplay();
  }

  @override
  void dispose() {
    _disposed = true;
    _invalidateRecordingSession();
    unawaited(_disposeRecorder());
    _answerController.dispose();
    _player?.dispose();
    super.dispose();
  }

  Future<void> _disposeRecorder() async {
    await _cleanupRecording(stopRecorder: true);
    await _recorder.dispose();
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
          builder: (context, _) => widget.controller.conversation == null
              ? _ScenarioPicker(
                  controller: widget.controller,
                  onStart: _start,
                  scenarioType: widget.scenarioType,
                )
              : _ConversationView(
                  controller: widget.controller,
                  answerController: _answerController,
                  scenario: _scenarioFor(widget.controller.conversation!),
                  onSend: _send,
                  onSpeak: _speak,
                  onStartRecording: _startPushToTalk,
                  onStopRecording: _finishPushToTalk,
                  onCancelRecording: _cancelPushToTalk,
                  recording: _recording,
                  voiceState: _voiceState,
                ),
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

  RoleplayScenario _scenarioFor(Conversation conversation) {
    for (final scenario in widget.controller.scenarios) {
      if (scenario.id == conversation.roleplayType) return scenario;
    }
    return RoleplayScenario(
      id: conversation.roleplayType,
      title: 'Current work conversation',
      partnerRole: 'AI partner',
      level: 'B1',
      context: conversation.context,
      goal: 'Explain the evidence, impact and next safe step.',
    );
  }

  Future<void> _startPushToTalk() async {
    if (_disposed ||
        _recording ||
        _starting ||
        _finishing ||
        _discarding ||
        widget.controller.working) {
      return;
    }
    final session = ++_recordingSession;
    _starting = true;
    _pressActive = true;
    _recordingLimitReached = false;
    if (mounted) {
      setState(() => _voiceState = 'Requesting microphone permission');
    }
    try {
      final allowed = await _recorder.hasPermission().timeout(
        _recordingStartTimeout,
      );
      if (!_isSessionActive(session) || !_pressActive) return;
      if (!allowed) {
        _pressActive = false;
        if (mounted) {
          setState(() => _voiceState = 'Permission denied — type a reply');
        }
        return;
      }
      _audio.clear();
      _recorderActive = true;
      final stream = await _recorder
          .startStream(
            const RecordConfig(
              encoder: AudioEncoder.pcm16bits,
              sampleRate: 16000,
              numChannels: 1,
              echoCancel: true,
              noiseSuppress: true,
            ),
          )
          .timeout(_recordingStartTimeout);
      if (!_isSessionActive(session) || !_pressActive) {
        await _cleanupRecording(stopRecorder: true, forceStop: true);
        return;
      }
      _recording = true;
      final drain = Completer<void>();
      _recordingDrain = drain;
      _recordingSubscription = stream.listen(
        (chunk) => _onAudioChunk(session, chunk),
        onError: (Object error, StackTrace stackTrace) {
          _onRecordingError(session);
          _completeDrain(drain);
        },
        onDone: () => _completeDrain(drain),
      );
      if (!_isSessionActive(session)) return;
      if (!_pressActive) {
        await _discardRecording(state: 'Ready');
        return;
      }
      _recordingTimer = Timer(_recordingMaxDuration, () {
        if (!_isSessionActive(session) || !_recording || _finishing) return;
        if (mounted) {
          setState(
            () => _voiceState = 'Recording limit reached — transcribing',
          );
        }
        unawaited(_finishPushToTalk());
      });
      if (mounted) {
        setState(() {
          _voiceState = 'Recording — release to transcribe';
        });
      }
    } catch (_) {
      await _cleanupRecording(stopRecorder: true);
      if (_isSessionActive(session) && mounted) {
        setState(() => _voiceState = 'Microphone unavailable — type a reply');
      }
    } finally {
      _starting = false;
    }
  }

  bool _isSessionActive(int session) =>
      !_disposed && session == _recordingSession && !_discarding;

  void _invalidateRecordingSession() {
    _recordingSession++;
    _pressActive = false;
    _recording = false;
    _recordingLimitReached = false;
    _recordingTimer?.cancel();
    _recordingTimer = null;
    _discarding = true;
    _completeDrain(_recordingDrain);
  }

  void _completeDrain(Completer<void>? drain) {
    if (drain != null && !drain.isCompleted) drain.complete();
  }

  void _onRecordingError(int session) {
    if (!_isSessionActive(session)) return;
    _invalidateRecordingSession();
    unawaited(
      _finishDiscardedRecording(
        state: 'Microphone stream failed — type a reply',
      ),
    );
  }

  void _onAudioChunk(int session, Uint8List chunk) {
    if (!_isSessionActive(session) || !_recording) return;
    final remaining = _recordingMaxBytes - _audio.length;
    if (remaining > 0) {
      _audio.addAll(chunk.take(remaining));
    }
    if (_audio.length >= _recordingMaxBytes &&
        !_recordingLimitReached &&
        !_finishing) {
      _recordingLimitReached = true;
      if (mounted) {
        setState(() => _voiceState = 'Recording limit reached — transcribing');
      }
      unawaited(_finishPushToTalk());
    }
  }

  Future<void> _finishPushToTalk() async {
    _pressActive = false;
    if (_disposed || _starting || !_recording || _finishing || _discarding) {
      return;
    }
    final session = _recordingSession;
    _finishing = true;
    try {
      if (!_isSessionActive(session)) return;
      if (mounted) setState(() => _voiceState = 'Transcribing');
      Object? stopError;
      var stopCompleted = false;
      try {
        await _stopRecorderBounded();
        stopCompleted = true;
        _recorderActive = false;
        if (_isSessionActive(session)) {
          try {
            await _recordingDrain?.future.timeout(widget.recordingStopTimeout);
          } catch (error) {
            stopError = error;
          }
        }
      } catch (error) {
        stopError = error;
      }
      if (!_isSessionActive(session)) return;
      final captured = stopError == null
          ? List<int>.from(_audio)
          : const <int>[];
      await _cleanupRecording(stopRecorder: !stopCompleted);
      if (!_isSessionActive(session)) return;
      if (stopError != null) {
        if (mounted) {
          setState(() => _voiceState = 'Recording could not stop — retry');
        }
        return;
      }
      if (captured.isEmpty) {
        if (mounted) {
          setState(() => _voiceState = 'No audio captured — type a reply');
        }
        return;
      }
      if (!_isSessionActive(session)) return;
      await widget.controller.transcribeAudio(
        _audioForUpload(captured),
        _audioMimeType,
      );
      if (!_isSessionActive(session) || !mounted) return;
      if (widget.controller.error != null) {
        setState(() => _voiceState = 'Transcript unavailable — type a reply');
        return;
      }
      final transcript = widget.controller.speakingSession?.transcript.trim();
      if (transcript == null || transcript.isEmpty) {
        setState(() => _voiceState = 'No transcript returned — type a reply');
        return;
      }
      _answerController.value = TextEditingValue(
        text: transcript,
        selection: TextSelection.collapsed(offset: transcript.length),
      );
      setState(() => _voiceState = 'Transcript ready — edit before sending');
    } finally {
      _finishing = false;
    }
  }

  Future<void> _cancelPushToTalk() async {
    if (_disposed) return;
    final hasRecordingWork =
        _starting ||
        _recording ||
        _finishing ||
        _recorderActive ||
        _recordingSubscription != null;
    _pressActive = false;
    if (!hasRecordingWork) {
      if (mounted) setState(() => _voiceState = 'Ready');
      return;
    }
    _invalidateRecordingSession();
    await _finishDiscardedRecording();
  }

  Future<void> _discardRecording({
    String state = 'Recording cancelled — type a reply',
  }) async {
    if (_disposed) return;
    _invalidateRecordingSession();
    await _finishDiscardedRecording(state: state);
  }

  Future<void> _finishDiscardedRecording({
    String state = 'Recording cancelled — type a reply',
  }) async {
    try {
      await _cleanupRecording(stopRecorder: true);
      if (!_disposed && mounted) setState(() => _voiceState = state);
    } finally {
      _discarding = false;
    }
  }

  Future<void> _cleanupRecording({
    required bool stopRecorder,
    bool forceStop = false,
  }) async {
    _recordingTimer?.cancel();
    _recordingTimer = null;
    final shouldStop = stopRecorder && (_recorderActive || forceStop);
    _recorderActive = false;
    if (shouldStop) {
      try {
        await _stopRecorderBounded();
      } catch (_) {
        // Stopping can fail when permission was denied or the stream already ended.
      } finally {
        _recorderActive = false;
      }
    }
    final drain = _recordingDrain;
    _recordingDrain = null;
    _completeDrain(drain);
    final subscription = _recordingSubscription;
    _recordingSubscription = null;
    if (subscription != null) {
      try {
        await subscription.cancel().timeout(const Duration(seconds: 2));
      } catch (_) {
        // Do not retain a stale subscription after a terminal recording path.
      }
    }
    _audio.clear();
    _recording = false;
    _recordingLimitReached = false;
  }

  Future<void> _stopRecorderBounded() {
    final pending = _recorderStopFuture;
    if (pending != null) {
      return pending.timeout(widget.recordingStopTimeout);
    }
    final operation = _recorder.stop();
    _recorderStopFuture = operation;
    unawaited(
      operation
          .whenComplete(() {
            if (identical(_recorderStopFuture, operation)) {
              _recorderStopFuture = null;
            }
          })
          .catchError((_) {}),
    );
    return operation.timeout(widget.recordingStopTimeout);
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

  String get _audioMimeType => 'audio/wav';

  List<int> _audioForUpload(List<int> audio) {
    final pcm = Uint8List.fromList(audio);
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
    required this.scenario,
    required this.onSend,
    required this.onSpeak,
    required this.onStartRecording,
    required this.onStopRecording,
    required this.onCancelRecording,
    required this.recording,
    required this.voiceState,
  });

  final AppController controller;
  final TextEditingController answerController;
  final RoleplayScenario scenario;
  final VoidCallback onSend;
  final ValueChanged<String> onSpeak;
  final VoidCallback onStartRecording;
  final VoidCallback onStopRecording;
  final VoidCallback onCancelRecording;
  final bool recording;
  final String voiceState;

  @override
  Widget build(BuildContext context) {
    final conversation = controller.conversation!;
    return Column(
      children: [
        Expanded(
          child: ListView.builder(
            padding: const EdgeInsets.all(AppSpacing.xl),
            itemCount: conversation.messages.length,
            itemBuilder: (context, index) {
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
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xl),
          child: LearningSupportCard(
            explanation:
                'Bối cảnh: ${scenario.context} Mục tiêu: ${scenario.goal} Bạn không cần nói hết một lần; hãy trả lời từng ý ngắn và dùng phần gợi ý khi bị bí.',
            starter:
                'First, I would explain the evidence. Then I would describe the impact and my next safe step.',
            followUp:
                'Hãy nói một chi tiết có thể kiểm tra được, rồi nêu bước tiếp theo.',
            onSpeak: onSpeak,
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
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Voice transcript is editable before sending.',
                key: const ValueKey('roleplay-transcript-hint'),
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: AppSpacing.sm),
              Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: TextField(
                      key: const ValueKey('roleplay-answer'),
                      controller: answerController,
                      minLines: 1,
                      maxLines: 4,
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
              const SizedBox(height: AppSpacing.sm),
              HoldToTalkButton(
                enabled: !controller.working || recording,
                recording: recording,
                onPressStart: onStartRecording,
                onPressEnd: onStopRecording,
                onPressCancel: onCancelRecording,
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'Voice: $voiceState',
                key: const ValueKey('roleplay-voice-state'),
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ],
          ),
        ),
      ],
    );
  }
}
