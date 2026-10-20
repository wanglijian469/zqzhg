import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  final base = Platform.environment['MOBILE18_INTEGRATION_BASE'];
  test(
    'Native model uses disposable MySQL for registration, drafts, media, SMS and administrative statistics',
    () async {
      // This file contains plain model tests: no widget binding or blocked HTTP override.
      expect(base, startsWith('http://127.0.0.1:'));
      SharedPreferences.setMockInitialValues({
        'base_url': base!,
        'endpoint_migration.aliyun_39_97_250_244_8089': true,
      });
      final m = AppModel(await SharedPreferences.getInstance(), {
        for (final lang in ['zh', 'en'])
          lang: asMap(
            jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
          ),
      }, analyticsEnabled: false);
      addTearDown(m.client.close);
      final caps = await m.clientCapabilities();
      expect(caps['product_name_only'], true);
      final name = 'native-entry-${DateTime.now().microsecondsSinceEpoch}',
          password = 'Native-Entry-Password-2026!';
      final registration = asMap(
        await m.api(
          '/auth/merchant/register',
          method: 'POST',
          data: {'login_name': name, 'password': password},
        ),
      );
      await m.signIn(registration);
      final companyAccount = m.account['id'];
      await expectLater(
        m.api(
          '/auth/merchant/register',
          method: 'POST',
          data: {'login_name': name, 'password': password},
        ),
        throwsA(
          isA<ApiFailure>().having(
            (e) => e.code,
            'duplicate account',
            'login_name_exists',
          ),
        ),
      );
      await expectLater(
        m.api(
          '/auth/merchant',
          method: 'POST',
          data: {'username': name, 'password': 'wrong-password'},
        ),
        throwsA(
          isA<ApiFailure>().having(
            (e) => e.code,
            'wrong password',
            'invalid_credentials',
          ),
        ),
      );
      await m.signOut();
      await m.signIn(
        asMap(
          await m.api(
            '/auth/merchant',
            method: 'POST',
            data: {'username': name, 'password': password},
          ),
        ),
      );
      expect(m.account['id'], companyAccount);
      final company = asMap(
        await m.api(
          '/merchant/contents',
          method: 'POST',
          data: {
            'kind': 'vendor',
            'data': {
              'name': '原生客户端验收企业（仅隔离测试）',
              'region_code': '130528',
              'main_products': '仅测试设备',
              'phone': '13900008888',
            },
          },
        ),
      );
      final product = asMap(
        await m.api(
          '/merchant/contents',
          method: 'POST',
          data: {
            'kind': 'product',
            'data': {'name': '名称唯一必填产品（仅隔离测试）'},
          },
        ),
      );
      final updated = asMap(
        await m.api(
          '/merchant/contents/${product['id']}/draft',
          method: 'PUT',
          data: {
            'revision_id': product['revision_id'],
            'data': {
              ...asMap(product['data']),
              'parameters': '型号：\n规格：',
              'description_en': '',
            },
          },
        ),
      );
      expect(updated['id'], product['id']);
      await expectLater(
        m.api(
          '/merchant/contents/${product['id']}/draft',
          method: 'PUT',
          data: {
            'revision_id': product['revision_id'],
            'data': asMap(product['data']),
          },
        ),
        throwsA(
          isA<ApiFailure>().having(
            (e) => e.code,
            'revision protection',
            'stale_revision',
          ),
        ),
      );
      final media = await m.upload(
        Platform.environment['MOBILE18_IMAGE_FIXTURE']!,
      );
      expect(media, greaterThan(0));
      await m.api(
        '/merchant/contents/${company['id']}/submit',
        method: 'POST',
        data: {'revision_id': company['revision_id']},
      );
      final stats = asMap(
        await m.api(
          '/public/industry-map/regions?hall=agriculture&region_code=1305',
        ),
      );
      expect(stats['total'], 2);
      expect(stats['unassigned_count'], 1);
      expect(
        rows(
          stats['items'],
        ).firstWhere((r) => r['code'] == '130528')['vendor_count'],
        1,
      );
      final county = asMap(
        await m.api(
          nativeIndustryVendorPath(
            'hall=agriculture&region_code=130528',
            1,
            false,
          ),
        ),
      );
      expect(county['total'], 1);
      expect(rows(county['items']), hasLength(1));
      final unassigned = asMap(
        await m.api(
          nativeIndustryVendorPath(
            'hall=agriculture&region_code=1305',
            1,
            true,
          ),
        ),
      );
      expect(unassigned['total'], 1);
      expect(rows(unassigned['items']), hasLength(1));
      final region = asMap(
        await m.api('/public/administrative-regions?code=130528'),
      );
      expect(rows(region['trail']).map((r) => r['code']), [
        '13',
        '1305',
        '130528',
      ]);
      await m.signOut();
      final sms = asMap(
        await m.api(
          '/auth/sms',
          method: 'POST',
          data: {'phone': '13900008888'},
        ),
      );
      await m.signIn(
        asMap(
          await m.api(
            '/auth/phone',
            method: 'POST',
            data: {'phone': '13900008888', 'code': sms['development_code']},
          ),
        ),
      );
      expect(m.account['id'], isNot(companyAccount));
      expect(
        m.prefs.getKeys().any(
          (key) => '${m.prefs.get(key)}'.contains(password),
        ),
        false,
      );
    },
    skip: base == null
        ? 'Use TestMobileUpgradeIntegration; never point this at a business database'
        : false,
  );
}
