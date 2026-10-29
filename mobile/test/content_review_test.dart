import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'app_navigation_test.dart' as f;
import 'free_service_test.dart' as service;

void main() {
  testWidgets(
    'Company editor confirms current agreements inline without identity fields or files',
    (t) async {
      bool accepted = false;
      int posts = 0;
      final m = await f.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/merchant/verification')) {
            return service.jsonResponse({
              'documents': service.docs,
              'acceptances': accepted
                  ? service.docs
                        .where((d) => d['key'] != 'service-info')
                        .map(
                          (d) => {
                            'document_id': d['id'],
                            'digest': d['digest'],
                          },
                        )
                        .toList()
                  : [],
            });
          }
          if (r.url.path.endsWith('/service-acceptances')) {
            posts++;
            final data = asMap(jsonDecode(r.body));
            expect(rows(data['agreements']), hasLength(3));
            accepted = true;
            return service.jsonResponse({'ok': true});
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.harness(
          m,
          home: const Scaffold(
            body: SingleChildScrollView(child: NativePublicationAgreements()),
          ),
        ),
      );
      await t.pumpAndSettle();
      expect(find.byType(TextField), findsNothing);
      expect(find.byType(CheckboxListTile), findsOneWidget);
      for (final label in ['法定代表人', '经办人', '营业执照', '经办授权证明']) {
        expect(find.text(label), findsNothing);
      }
      final check = find.byType(CheckboxListTile);
      await t.ensureVisible(check);
      await t.tap(check);
      await t.pumpAndSettle();
      final submit = find.widgetWithText(FilledButton, '确认协议');
      await t.ensureVisible(submit);
      await t.tap(submit);
      await t.pumpAndSettle();
      expect(posts, 1);
      expect(find.text('已确认当前版本协议'), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Legacy verification entry opens company agreement step instead of a separate identity form',
    (t) async {
      final m = await f.fixture(
        intercept: (r) async => r.url.path.endsWith('/merchant/verification')
            ? service.jsonResponse({
                'documents': service.docs,
                'acceptances': [],
              })
            : null,
      );
      await t.pumpWidget(f.harness(m, home: const NativeVerificationScreen()));
      await t.pumpAndSettle();
      expect(find.widgetWithText(ChoiceChip, '3. 协议与提交'), findsOneWidget);
      expect(find.byType(NativePublicationAgreements), findsOneWidget);
      expect(find.text('提交核验'), findsNothing);
      expect(find.text('营业执照'), findsNothing);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
}
