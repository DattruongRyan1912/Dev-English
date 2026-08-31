/// Canonical workspace feature composition.
///
/// Screen implementations live under their owning feature directories and
/// share private visual, controller, and dialog primitives through this
/// composition library. Public Today/Work/Knowledge/Learning entry points are
/// kept separate from the legacy screen compatibility area.
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';
import 'package:record/record.dart';

import '../../app_controller.dart';
import '../../components.dart';
import '../../learning_overlay_copy.dart';
import '../../theme.dart';
import '../../workspace_controller.dart';
import '../../workspace_models.dart';

part 'workspace_shared.dart';
part '../today/workspace_today.dart';
part '../assistant/workspace_assistant.dart';
part '../work/workspace_work.dart';
part '../knowledge/workspace_knowledge.dart';
part '../learning/workspace_learning.dart';
part 'workspace_dialogs_work.dart';
part 'workspace_dialogs_actions.dart';
part 'workspace_dialogs_import.dart';
part 'workspace_details.dart';
