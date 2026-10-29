part of 'main.dart';

class LoginScreen extends StatelessWidget {
  final bool visitor, register, binding, companyAccount;
  final String initialName;
  final VoidCallback? onRegistered;
  const LoginScreen({
    super.key,
    this.visitor = false,
    this.register = false,
    this.binding = false,
    this.companyAccount = true,
    this.initialName = '',
    this.onRegistered,
  });
  @override
  Widget build(BuildContext context) => binding
      ? LegacyPhoneBindingScreen(binding: true)
      : UnifiedAuthScreen(register: register, initialName: initialName);
}

class UnifiedAuthScreen extends StatefulWidget {
  final bool register;
  final String initialName;
  const UnifiedAuthScreen({
    super.key,
    this.register = false,
    this.initialName = '',
  });
  @override
  State<UnifiedAuthScreen> createState() => _UnifiedAuthScreenState();
}

class _UnifiedAuthScreenState extends State<UnifiedAuthScreen> {
  final fields = {
    for (final k in [
      'phone',
      'code',
      'username',
      'phonePassword',
      'phoneConfirm',
      'accountPassword',
      'accountConfirm',
      'loginPassword',
      'resetPassword',
      'resetConfirm',
    ])
      k: TextEditingController(),
  };
  bool developmentSMS = false;
  bool registration = false,
      accountTab = false,
      reset = false,
      busy = false,
      shown = false,
      confirmShown = false;
  int step = 1, seconds = 0;
  Timer? timer;
  String action = '', error = '', notice = '', requestID = '', dev = '';
  List<Json> agreements = [];
  int agreementKey = 0;
  String value(String k) => fields[k]!.text;
  String tr(String z, String e) => syncText(context, z, e);
  String get purpose => reset ? 'reset_password' : 'register';
  String get passwordKey => reset
      ? 'resetPassword'
      : registration
      ? (accountTab ? 'accountPassword' : 'phonePassword')
      : 'loginPassword';
  String get confirmKey => reset
      ? 'resetConfirm'
      : accountTab
      ? 'accountConfirm'
      : 'phoneConfirm';
  bool get mobile => reset || registration && !accountTab;
  @override
  void initState() {
    super.initState();
    registration = widget.register;
    fields['username']!.text = widget.initialName;
  }

  @override
  void dispose() {
    timer?.cancel();
    for (final f in fields.values) {
      f.dispose();
    }
    super.dispose();
  }

  String failure(String code) =>
      {
        'invalid_credentials': tr(
          '手机号、用户名或密码不正确',
          'The account or password is incorrect',
        ),
        'invalid_code': tr('验证码错误或已过期', 'Incorrect or expired code'),
        'phone_action_expired': tr(
          '验证已过期，请返回重新验证手机号',
          'Verification expired. Verify your mobile again.',
        ),
        'phone_exists': tr(
          '该手机号已注册，请登录或找回密码',
          'Already registered. Sign in or recover your password.',
        ),
        'phone_reset_unavailable': tr(
          '未找到已验证的手机号，请检查或联系管理员',
          'No verified number found. Contact support.',
        ),
        'password_length': tr('请输入8–20位密码', 'Enter an 8–20 character password'),
        'agreement_required': tr(
          '请阅读并同意用户协议和隐私政策',
          'Accept the terms and privacy policy',
        ),
        'sms_login_disabled': tr(
          '请使用密码登录，忘记密码可用手机号找回',
          'Use password sign-in or mobile password recovery',
        ),
      }[code] ??
      ModelScope.of(context).t(code);
  Future<Json?> proof(AppModel m) async {
    Json image = asMap(
      await m.api(
        '/auth/phone/challenge',
        method: 'POST',
        data: {'phone': value('phone').trim(), 'purpose': purpose},
      ),
    );
    if (!mounted) return null;
    String answer = '';
    bool loading = false;
    String problem = '';
    return showDialog<Json>(
      context: context,
      builder: (dialog) => StatefulBuilder(
        builder: (c, change) => AlertDialog(
          title: Text(tr('图形验证码', 'Image verification')),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                if ((image['image'] ?? '').toString().startsWith(
                  'data:image/png;base64,',
                ))
                  Image.memory(
                    base64Decode(image['image'].split(',').last),
                    width: 156,
                    height: 52,
                  ),
                TextField(
                  key: ValueKey('sms-image-answer-${image['challenge_id']}'),
                  onChanged: (s) => answer = s,
                  keyboardType: TextInputType.number,
                  inputFormatters: [
                    FilteringTextInputFormatter.digitsOnly,
                    LengthLimitingTextInputFormatter(5),
                  ],
                  decoration: InputDecoration(
                    labelText: tr('输入图片中的五位数字', 'Enter the five digits'),
                  ),
                ),
                if (problem.isNotEmpty) Text(problem),
                TextButton(
                  onPressed: loading
                      ? null
                      : () async {
                          change(() => loading = true);
                          try {
                            final next = asMap(
                              await m.api(
                                '/auth/phone/challenge',
                                method: 'POST',
                                data: {
                                  'phone': value('phone').trim(),
                                  'purpose': purpose,
                                },
                              ),
                            );
                            if (c.mounted) {
                              change(() {
                                image = next;
                                answer = '';
                                problem = '';
                              });
                            }
                          } catch (_) {
                            if (c.mounted) {
                              change(
                                () => problem = tr(
                                  '加载失败，请重试',
                                  'Could not load. Retry.',
                                ),
                              );
                            }
                          } finally {
                            if (c.mounted) change(() => loading = false);
                          }
                        },
                  child: Text(tr('换一张', 'Refresh')),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialog),
              child: Text(tr('取消', 'Cancel')),
            ),
            FilledButton(
              onPressed: loading
                  ? null
                  : () {
                      if (!RegExp(r'^\d{5}$').hasMatch(answer)) {
                        change(
                          () => problem = tr(
                            '请输入五位图片验证码',
                            'Enter the five digits',
                          ),
                        );
                        return;
                      }
                      Navigator.pop(dialog, {
                        'challenge_id': image['challenge_id'],
                        'challenge_answer': answer,
                      });
                    },
              child: Text(tr('验证并发送', 'Verify and send')),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> run({bool send = false}) async {
    if (busy) return;
    final m = ModelScope.of(context), source = m.baseUrl;
    setState(() {
      error = '';
      notice = '';
    });
    if (mobile && !RegExp(r'^1[3-9]\d{9}$').hasMatch(value('phone').trim())) {
      setState(() => error = tr('请输入正确的手机号', 'Enter a valid mobile number'));
      return;
    }
    if (!send &&
        mobile &&
        step == 1 &&
        !RegExp(r'^\d{6}$').hasMatch(value('code'))) {
      setState(() => error = tr('请输入六位短信验证码', 'Enter the six-digit code'));
      return;
    }
    if (!send && (!mobile || step == 2)) {
      if (!mobile && value('username').trim().isEmpty) {
        setState(
          () => error = tr('请输入手机号或用户名', 'Enter a mobile number or username'),
        );
        return;
      }
      if (registration || reset) {
        if (!validNewPassword(value(passwordKey))) {
          setState(
            () => error = tr('请输入8–20位密码', 'Enter an 8–20 character password'),
          );
          return;
        }
        if (value(passwordKey) != value(confirmKey)) {
          setState(
            () => error = tr('两次输入的密码不一致', 'The passwords do not match'),
          );
          return;
        }
        if (registration && agreements.length != 2) {
          setState(() => error = failure('agreement_required'));
          return;
        }
      } else if (value(passwordKey).isEmpty) {
        setState(() => error = tr('请输入密码', 'Enter your password'));
        return;
      }
    }
    setState(() => busy = true);
    try {
      if (send) {
        if (seconds > 0) return;
        Json p = {};
        final caps = await m.clientCapabilities();
        developmentSMS = caps['development_sms'] == true;
        if (caps['sms_challenge'] == true) {
          final result = await proof(m);
          if (result == null) return;
          p = result;
        }
        if (!mounted || source != m.baseUrl) return;
        requestID = requestID.isEmpty ? newServiceCredential() : requestID;
        final r = asMap(
          await m.api(
            '/auth/phone/code',
            method: 'POST',
            data: {
              'phone': value('phone').trim(),
              'purpose': purpose,
              ...p,
              'request_id': requestID,
            },
          ),
        );
        if (!mounted || source != m.baseUrl) return;
        requestID = '';
        setState(() {
          seconds = (r['retry_after'] ?? 60) as int;
          dev = (r['development_code'] ?? '').toString();
        });
        timer?.cancel();
        timer = Timer.periodic(const Duration(seconds: 1), (t) {
          if (!mounted) {
            t.cancel();
            return;
          }
          setState(() => seconds = seconds > 0 ? seconds - 1 : 0);
          if (seconds == 0) t.cancel();
        });
        return;
      }
      if (mobile && step == 1) {
        final r = asMap(
          await m.api(
            '/auth/phone/verify',
            method: 'POST',
            data: {
              'phone': value('phone').trim(),
              'purpose': purpose,
              'code': value('code'),
            },
          ),
        );
        if (mounted && source == m.baseUrl) {
          setState(() {
            action = r['action_token'];
            step = 2;
          });
        }
        return;
      }
      if (reset) {
        await m.api(
          '/auth/phone/password/reset',
          method: 'POST',
          data: {
            'phone': value('phone').trim(),
            'action_token': action,
            'password': value(passwordKey),
          },
        );
        if (!mounted || source != m.baseUrl) return;
        setState(() {
          fields['username']!.text = value('phone').trim();
          fields['resetPassword']!.clear();
          fields['resetConfirm']!.clear();
          fields['loginPassword']!.clear();
          reset = false;
          step = 1;
          action = '';
          notice = tr(
            '密码已重置，请使用新密码登录',
            'Password reset. Sign in with your new password.',
          );
        });
        return;
      }
      final path = registration
          ? (accountTab ? '/auth/visitor/register' : '/auth/phone/register')
          : '/auth/password';
      final data = registration
          ? {
              'password': value(passwordKey),
              'agreements': agreements,
              'source': Platform.isIOS ? 'ios' : 'android',
              if (accountTab)
                'login_name': value('username').trim()
              else ...{
                'phone': value('phone').trim(),
                'action_token': action,
              },
            }
          : {
              'username': value('username').trim(),
              'password': value(passwordKey),
            };
      final result = asMap(await m.api(path, method: 'POST', data: data));
      if (!mounted || source != m.baseUrl) return;
      await m.signIn(result);
      if (!mounted) return;
      if (registration) {
        m.openHome?.call();
        Navigator.of(context).popUntil((r) => r.isFirst);
      } else {
        Navigator.pop(context, true);
      }
    } catch (e) {
      if (!mounted || source != m.baseUrl) return;
      final f = e is ApiFailure ? e : ApiFailure('network_error');
      setState(() {
        error = failure(f.code);
        if (f.code == 'phone_action_expired') {
          step = 1;
          action = '';
        }
        if (f.code == 'agreement_required') {
          agreements = [];
          agreementKey++;
        }
        if (send && f.code != 'network_error') requestID = '';
      });
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Widget field(
    String key,
    String label, {
    bool secret = false,
    bool confirm = false,
  }) => Padding(
    padding: const EdgeInsets.only(bottom: 14),
    child: TextField(
      key: ValueKey('auth-$key'),
      controller: fields[key],
      enabled: !busy,
      obscureText: secret && !(confirm ? confirmShown : shown),
      keyboardType: key == 'phone'
          ? TextInputType.phone
          : key == 'code'
          ? TextInputType.number
          : null,
      autocorrect: false,
      enableSuggestions: false,
      onChanged: (v) {
        if (key == 'phone') {
          action = '';
          requestID = '';
          fields['code']!.clear();
        }
      },
      decoration: InputDecoration(
        labelText: label,
        border: const OutlineInputBorder(),
        suffixIcon: secret
            ? IconButton(
                tooltip: (confirm ? confirmShown : shown)
                    ? tr('隐藏$label', 'Hide $label')
                    : tr('显示$label', 'Show $label'),
                onPressed: () => setState(
                  () => confirm ? confirmShown = !confirmShown : shown = !shown,
                ),
                icon: Icon(
                  (confirm ? confirmShown : shown)
                      ? Icons.visibility_off_outlined
                      : Icons.visibility_outlined,
                ),
              )
            : null,
      ),
    ),
  );
  @override
  Widget build(BuildContext context) {
    final title = reset
        ? tr('找回密码', 'Recover password')
        : registration
        ? tr('注册中企展馆', 'Register')
        : tr('登录中企展馆', 'Sign in');
    final intro = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            fontSize: 28,
            fontWeight: FontWeight.bold,
            color: industrialNavy,
          ),
        ),
        const SizedBox(height: 12),
        Text(
          tr(
            '免费浏览企业与产品，也可使用同一账号申请企业展厅。',
            'Explore companies and products, or apply for a free showroom with the same account.',
          ),
        ),
      ],
    );
    final form = Card(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (registration)
              Padding(
                padding: const EdgeInsets.only(bottom: 18),
                child: Row(
                  children: [
                    for (final account in [false, true])
                      Expanded(
                        child: TextButton(
                          onPressed: busy
                              ? null
                              : () => setState(() {
                                  accountTab = account;
                                  error = '';
                                }),
                          style: TextButton.styleFrom(
                            backgroundColor: accountTab == account
                                ? const Color(0xffeaf2fe)
                                : null,
                          ),
                          child: Text(
                            account
                                ? tr('账号密码注册', 'Username')
                                : tr('手机验证码注册', 'Mobile'),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            if (reset)
              Padding(
                padding: const EdgeInsets.only(bottom: 14),
                child: Text(
                  tr(
                    '使用已验证的手机号找回密码。未绑定手机号请联系管理员。',
                    'Use your verified mobile. Contact support if no number is linked.',
                  ),
                ),
              ),
            if (mobile && step == 1) ...[
              field('phone', tr('手机号', 'Mobile number')),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: field('code', tr('短信验证码', 'SMS verification code')),
                  ),
                  const SizedBox(width: 8),
                  OutlinedButton(
                    onPressed: busy || seconds > 0
                        ? null
                        : () => run(send: true),
                    child: Text(
                      seconds > 0 ? '${seconds}s' : tr('获取验证码', 'Get code'),
                    ),
                  ),
                ],
              ),
            ] else ...[
              if (!mobile)
                field(
                  'username',
                  registration
                      ? tr('用户名', 'Username')
                      : tr('手机号或用户名', 'Mobile number or username'),
                ),
              if (mobile)
                Padding(
                  padding: const EdgeInsets.only(bottom: 14),
                  child: Text(
                    tr('手机号已验证，请设置密码', 'Mobile verified. Set your password.'),
                  ),
                ),
              field(
                passwordKey,
                registration || reset
                    ? tr('设置密码', 'Set password')
                    : tr('密码', 'Password'),
                secret: true,
              ),
              if (registration || reset) ...[
                Padding(
                  padding: const EdgeInsets.only(bottom: 14),
                  child: Text(
                    tr(
                      authPasswordPolicy['hint_zh']!,
                      authPasswordPolicy['hint_en']!,
                    ),
                  ),
                ),
                field(
                  confirmKey,
                  tr('确认密码', 'Confirm password'),
                  secret: true,
                  confirm: true,
                ),
              ],
              if (registration)
                NativeRegistrationAgreements(
                  key: ValueKey(agreementKey),
                  initiallyAccepted: agreements.length == 2,
                  onChanged: (refs) => agreements = refs,
                ),
            ],
            if (dev.isNotEmpty && developmentSMS)
              Text('${tr('本机模拟验证码：', 'Mock code: ')}$dev'),
            if (error.isNotEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Text(
                  error,
                  key: const ValueKey('auth-error'),
                  style: const TextStyle(color: Colors.red),
                ),
              ),
            if (notice.isNotEmpty) Text(notice),
            FilledButton(
              onPressed: busy ? null : () => run(),
              child: Text(
                busy
                    ? tr('处理中…', 'Processing…')
                    : mobile && step == 1
                    ? tr('下一步', 'Next')
                    : reset
                    ? tr('确认重置密码', 'Reset password')
                    : registration
                    ? tr('注册并进入展馆', 'Register and enter')
                    : tr('登录', 'Sign in'),
              ),
            ),
            if (mobile && step == 2)
              TextButton(
                onPressed: busy
                    ? null
                    : () => setState(() {
                        step = 1;
                        action = '';
                      }),
                child: Text(tr('返回验证手机号', 'Back to verification')),
              ),
            if (!registration && !reset)
              TextButton(
                onPressed: busy
                    ? null
                    : () => setState(() {
                        reset = true;
                        step = 1;
                        error = '';
                        fields['code']!.clear();
                      }),
                child: Text(tr('忘记密码？', 'Forgot password?')),
              ),
            TextButton(
              onPressed: busy
                  ? null
                  : () => setState(() {
                      if (reset) {
                        reset = false;
                      } else {
                        registration = !registration;
                      }
                      error = '';
                    }),
              child: Text(
                reset
                    ? tr('返回密码登录', 'Back to sign-in')
                    : registration
                    ? tr('已有账号，登录', 'Already registered? Sign in')
                    : tr('免费注册', 'Register free'),
              ),
            ),
          ],
        ),
      ),
    );
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, c) => SingleChildScrollView(
            padding: const EdgeInsets.all(20),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 1120),
                child: c.maxWidth >= 960
                    ? Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Expanded(
                            child: Padding(
                              padding: const EdgeInsets.all(28),
                              child: intro,
                            ),
                          ),
                          const SizedBox(width: 36),
                          SizedBox(width: 430, child: form),
                        ],
                      )
                    : Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [intro, const SizedBox(height: 20), form],
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
