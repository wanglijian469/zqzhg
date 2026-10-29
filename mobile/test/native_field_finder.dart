import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Finder nativeField(String label) => find.descendant(
  of: find.widgetWithText(NativeFieldLabel, label),
  matching: find.byType(TextField),
);
Finder nativeFormField(String label) => find.descendant(
  of: find.widgetWithText(NativeFieldLabel, label),
  matching: find.byType(TextFormField),
);
