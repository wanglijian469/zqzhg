part of 'main.dart';

class LoginScreen extends StatefulWidget {
  final bool visitor, register, binding;
  final String initialName;
  final VoidCallback? onRegistered;
  const LoginScreen({
    super.key,
    this.visitor = false,
    this.register = false,
    this.binding = false,
    this.initialName = '',
    this.onRegistered,
  });
  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final controls = {
    for (final k in ['phone', 'code', 'password', 'confirmation', 'login_name'])
      k: TextEditingController(),
  };
  final focuses = {
    for (final k in ['phone', 'code', 'password', 'confirmation', 'login_name'])
      k: FocusNode(),
  };
  bool registerMode = false,
      legacy = false,
      setup = false,
      busy = false,
      visible = false,
      confirmationVisible = false,
      unconfirmed = false;
  Map<String, String> errors = {};
  String error = '', dev = '';
  int cooldown = 0;
  Timer? timer;
  bool get verifying => widget.binding || registerMode || setup;
  String value(String k) => controls[k]!.text;
  String text(String z, String e) => syncText(context, z, e);
  @override
  void initState() {
    super.initState();
    registerMode = widget.register;
    controls['login_name']!.text = widget.initialName;
  }

  @override
  void dispose() {
    for (final c in controls.values) {
      c.dispose();
    }
    for (final f in focuses.values) {
      f.dispose();
    }
    timer?.cancel();
    super.dispose();
  }

  void mode({
    bool registration = false,
    bool old = false,
    bool setting = false,
  }) {
    if (busy) return;
    setState(() {
      registerMode = registration;
      legacy = old;
      setup = setting;
      for (final k in ['password', 'confirmation', 'code']) {
        controls[k]!.clear();
      }
      visible = confirmationVisible = unconfirmed = false;
      errors = {};
      error = dev = '';
    });
  }

  void focusErrors() {
    final key = [
      'phone',
      'login_name',
      'code',
      'password',
      'confirmation',
    ].firstWhere(errors.containsKey, orElse: () => 'phone');
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final n = focuses[key]!;
      n.requestFocus();
      if (n.context != null) {
        unawaited(
          Scrollable.ensureVisible(
            n.context!,
            alignment: .25,
            duration: const Duration(milliseconds: 200),
          ),
        );
      }
    });
  }

  Future<void> submit({bool send = false}) async {
    if (busy || send && cooldown > 0 || !send && unconfirmed) return;
    final m = ModelScope.of(context),
        source = m.baseUrl,
        invalid = <String, String>{};
    if (legacy) {
      if (value('login_name').trim().isEmpty ||
          value('login_name').trim().runes.length > 100) {
        invalid['login_name'] = text(
          '账号不能为空，最多100字',
          'Required, up to 100 characters',
        );
      }
    } else if (!RegExp(r'^1[3-9][0-9]{9}$').hasMatch(value('phone').trim())) {
      invalid['phone'] = text(
        '请填写中国大陆11位手机号',
        'Enter an 11-digit mainland China mobile number',
      );
    }
    if (!send) {
      if (verifying && !RegExp(r'^\d{6}$').hasMatch(value('code').trim())) {
        invalid['code'] = text('请填写6位验证码', 'Enter the 6-digit code');
      }
      if (!widget.binding) {
        final n = utf8.encode(value('password')).length;
        if ((registerMode || setup) &&
            (n < 12 || n > 72 || value('password').trim().isEmpty)) {
          invalid['password'] = text(
            '密码需12–72个UTF-8字节',
            'Use 12–72 UTF-8 bytes',
          );
        } else if (value('password').isEmpty) {
          invalid['password'] = text('请填写密码', 'Enter your password');
        }
        if ((registerMode || setup) &&
            value('password') != value('confirmation')) {
          invalid['confirmation'] = text('两次密码不一致', 'Passwords do not match');
        }
      }
    }
    setState(() {
      errors = invalid;
      error = '';
    });
    if (invalid.isNotEmpty) {
      focusErrors();
      return;
    }
    setState(() => busy = true);
    bool registrationRequestStarted = false;
    try {
      if (!legacy) {
        var capabilities = await m.clientCapabilities();
        if (capabilities['phone_password_auth'] != true) {
          capabilities = await m.clientCapabilities(refresh: true);
        }
        if (capabilities['phone_password_auth'] != true) {
          throw ApiFailure('phoneAuthUpgradeRequired');
        }
      }
      if (!mounted || m.baseUrl != source) return;
      final path = legacy
          ? (widget.visitor ? '/auth/visitor' : '/auth/merchant')
          : widget.binding
          ? (send ? '/auth/me/phone/code' : '/auth/me/phone')
          : send
          ? '/auth/phone/code'
          : setup
          ? '/auth/phone/password/setup'
          : registerMode
          ? '/auth/phone/register'
          : '/auth/phone/password';
      registrationRequestStarted = registerMode && !send;
      final result = asMap(
        await m.api(
          path,
          method: widget.binding && !send ? 'PUT' : 'POST',
          data: {
            if (legacy)
              'username': value('login_name').trim()
            else
              'phone': value('phone').trim(),
            if (send && !widget.binding)
              'purpose': setup ? 'setup_password' : 'register',
            if (!send && verifying) 'code': value('code').trim(),
            if (!send && !widget.binding) 'password': value('password'),
            if (!send && registerMode)
              'role': widget.visitor ? 'visitor' : 'merchant',
          },
        ),
      );
      if (!mounted || source != m.baseUrl) return;
      if (send) {
        setState(() {
          dev = (result['development_code'] ?? '').toString();
          cooldown = 60;
        });
        timer?.cancel();
        timer = Timer.periodic(const Duration(seconds: 1), (t) {
          if (!mounted) {
            t.cancel();
            return;
          }
          setState(() => cooldown--);
          if (cooldown <= 0) t.cancel();
        });
      } else {
        await m.signIn(result);
        controls['password']!.clear();
        controls['confirmation']!.clear();
        if (!mounted || source != m.baseUrl) return;
        if (widget.binding) {
          Navigator.pop(context, true);
          return;
        }
        if (legacy && m.account['requires_phone_binding'] == true) {
          final capable =
              (await m.clientCapabilities())['phone_password_auth'] == true;
          if (!mounted) return;
          if (capable) {
            await Navigator.push<bool>(
              context,
              MaterialPageRoute(
                builder: (_) => const LoginScreen(binding: true),
              ),
            );
          }
        }
        if (!mounted || source != m.baseUrl) return;
        if (!widget.visitor && m.account['role'] == 'visitor') {
          if (!await confirmMerchantUpgrade(context)) return;
          if (!mounted) return;
          widget.onRegistered?.call();
        } else if (registerMode && !widget.visitor) {
          widget.onRegistered?.call();
        }
        if (mounted) Navigator.pop(context, true);
      }
    } catch (e) {
      if (!mounted || source != m.baseUrl) return;
      final f = e is ApiFailure ? e : ApiFailure('network_error');
      setState(() {
        if (f.fields.isNotEmpty) {
          errors = {for (final x in f.fields) x['field']: fieldIssueText(m, x)};
        } else {
          error = m.t(f.code);
        }
        if ([
          'phone_exists',
          'phone_already_bound',
          'password_setup_unavailable',
        ].contains(f.code)) {
          error = m.t(f.code);
        }
        if (f.code == 'invalid_code') {
          errors['code'] = text(
            '验证码错误、已过期或已使用，请重新获取',
            'The code is incorrect, expired or already used. Request a new code.',
          );
        }
        if (registrationRequestStarted && f.code == 'network_error') {
          unconfirmed = true;
          error = text(
            '无法确认账号是否已注册。请用该手机号和刚设置的密码登录继续。',
            'Registration could not be confirmed. Sign in with this mobile number and the password you just set.',
          );
        }
      });
      if (errors.isNotEmpty) focusErrors();
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Widget input(
    String key,
    String label, {
    bool secret = false,
    bool confirm = false,
  }) => TextField(
    key: ValueKey('auth-$key'),
    controller: controls[key],
    focusNode: focuses[key],
    enabled: !busy,
    obscureText: secret && !(confirm ? confirmationVisible : visible),
    keyboardType: key == 'phone'
        ? TextInputType.phone
        : key == 'code'
        ? TextInputType.number
        : null,
    autocorrect: false,
    enableSuggestions: false,
    inputFormatters: key == 'phone'
        ? [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(11),
          ]
        : key == 'code'
        ? [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(6),
          ]
        : null,
    textInputAction: key == 'confirmation' || !verifying && key == 'password'
        ? TextInputAction.done
        : TextInputAction.next,
    onSubmitted: (_) => key == 'confirmation' || !verifying && key == 'password'
        ? submit()
        : FocusScope.of(context).nextFocus(),
    autofillHints: key == 'phone'
        ? [AutofillHints.telephoneNumberNational]
        : key == 'code'
        ? [AutofillHints.oneTimeCode]
        : key == 'login_name'
        ? [AutofillHints.username]
        : null,
    onChanged: (_) => setState(() {
      errors.remove(key);
      if (key == 'phone') {
        controls['code']!.clear();
        dev = '';
      }
    }),
    decoration: InputDecoration(
      labelText: label,
      errorText: errors[key],
      errorMaxLines: 4,
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 16),
      suffixIcon: secret
          ? IconButton(
              tooltip: (confirm ? confirmationVisible : visible)
                  ? text('隐藏$label', 'Hide $label')
                  : text('显示$label', 'Show $label'),
              onPressed: () => setState(() {
                if (confirm) {
                  confirmationVisible = !confirmationVisible;
                } else {
                  visible = !visible;
                }
              }),
              icon: Icon(
                (confirm ? confirmationVisible : visible)
                    ? Icons.visibility_off_outlined
                    : Icons.visibility_outlined,
              ),
            )
          : null,
    ),
  );
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final title = widget.binding
        ? text('绑定手机号', 'Link mobile number')
        : setup
        ? text('原短信账号设置密码', 'Set password for SMS account')
        : legacy
        ? text('原账号登录', 'Original account sign-in')
        : widget.visitor
        ? (registerMode
              ? text('访客注册', 'Visitor registration')
              : text('访客登录', 'Visitor sign-in'))
        : registerMode
        ? text('厂商注册', 'Manufacturer registration')
        : text('厂商登录', 'Manufacturer sign-in');
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(context, title),
        body: SafeArea(
          top: false,
          child: SingleChildScrollView(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            padding: const EdgeInsets.all(16),
            child: Align(
              alignment: Alignment.topCenter,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 440),
                child: Card(
                  margin: EdgeInsets.zero,
                  child: Padding(
                    padding: const EdgeInsets.all(20),
                    child: AutofillGroup(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Text(
                            widget.binding
                                ? text(
                                    '验证后可用手机号登录，原账号与资料保留。可稍后在账号设置中完成。',
                                    'Verify your mobile number to sign in. Your account and data stay together. You may do this later in account settings.',
                                  )
                                : setup
                                ? text(
                                    '仅用于尚未设置密码的原短信账号。',
                                    'For original SMS accounts without a password.',
                                  )
                                : widget.visitor
                                ? text(
                                    '收藏优质厂商与产品，跨设备同步。',
                                    'Save manufacturers and products, and sync across devices.',
                                  )
                                : registerMode
                                ? text(
                                    '验证手机号 → 完善企业资料 → 提交审核',
                                    'Verify mobile → Complete profile → Submit for review',
                                  )
                                : text(
                                    '登录后管理企业展厅与产品。',
                                    'Manage your company showroom and products.',
                                  ),
                            style: const TextStyle(
                              color: Color(0xff637b95),
                              height: 1.5,
                            ),
                          ),
                          space(20),
                          input(
                            legacy ? 'login_name' : 'phone',
                            legacy
                                ? text('原登录账号', 'Original login name')
                                : text('手机号', 'Mobile number'),
                          ),
                          if (verifying) ...[
                            space(16),
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(
                                  child: input(
                                    'code',
                                    text('短信验证码', 'SMS code'),
                                  ),
                                ),
                                const SizedBox(width: 8),
                                ConstrainedBox(
                                  constraints: const BoxConstraints(
                                    minHeight: 56,
                                    maxWidth: 124,
                                  ),
                                  child: OutlinedButton(
                                    style: OutlinedButton.styleFrom(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 10,
                                        vertical: 14,
                                      ),
                                    ),
                                    onPressed: busy || cooldown > 0
                                        ? null
                                        : () => submit(send: true),
                                    child: Text(
                                      cooldown > 0
                                          ? '${cooldown}s'
                                          : text('发送验证码', 'Send code'),
                                      textAlign: TextAlign.center,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                            if (dev.isNotEmpty) Text('${m.t('devCode')}: $dev'),
                          ],
                          if (!widget.binding) ...[
                            space(16),
                            input(
                              'password',
                              (registerMode || setup)
                                  ? text('设置密码', 'Set password')
                                  : text('密码', 'Password'),
                              secret: true,
                            ),
                            if (registerMode || setup) ...[
                              space(16),
                              input(
                                'confirmation',
                                text('确认密码', 'Confirm password'),
                                secret: true,
                                confirm: true,
                              ),
                              space(12),
                              Text(
                                text(
                                  '密码12–72个UTF-8字节，不能全为空白。',
                                  'Password: 12–72 UTF-8 bytes; not all whitespace.',
                                ),
                                style: const TextStyle(
                                  fontSize: 12,
                                  color: Color(0xff637b95),
                                ),
                              ),
                            ],
                          ],
                          if (error.isNotEmpty)
                            Padding(
                              padding: const EdgeInsets.only(top: 12),
                              child: Text(
                                error,
                                style: const TextStyle(color: Colors.red),
                              ),
                            ),
                          space(20),
                          FilledButton(
                            style: FilledButton.styleFrom(
                              minimumSize: const Size(0, 48),
                            ),
                            onPressed: busy || unconfirmed
                                ? null
                                : () => submit(),
                            child: Text(
                              busy
                                  ? m.t('loading')
                                  : widget.binding
                                  ? text('验证并绑定', 'Verify and link')
                                  : setup
                                  ? text('设置密码并登录', 'Set password and sign in')
                                  : registerMode
                                  ? text('注册并继续', 'Register and continue')
                                  : text('登录', 'Sign in'),
                            ),
                          ),
                          if (widget.binding)
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () => Navigator.pop(context, false),
                              child: Text(text('暂时跳过', 'Skip for now')),
                            )
                          else ...[
                            space(8),
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () => mode(
                                      registration:
                                          !registerMode && !legacy && !setup,
                                    ),
                              child: Text(
                                unconfirmed
                                    ? text('已有账号？登录继续', 'Sign in to continue')
                                    : registerMode
                                    ? text(
                                        '已有账号？登录',
                                        'Already registered? Sign in',
                                      )
                                    : text(
                                        '注册${widget.visitor ? '访客' : '厂商'}账号',
                                        widget.visitor
                                            ? 'Register visitor account'
                                            : 'Register manufacturer account',
                                      ),
                              ),
                            ),
                            if (!registerMode && !setup && !legacy)
                              TextButton(
                                onPressed: busy ? null : () => mode(old: true),
                                child: Text(
                                  text('原账号登录', 'Original account sign-in'),
                                ),
                              ),
                            if (legacy || setup)
                              TextButton(
                                onPressed: busy ? null : () => mode(),
                                child: Text(
                                  text('手机号密码登录', 'Mobile password sign-in'),
                                ),
                              ),
                            if (!registerMode)
                              ExpansionTile(
                                tilePadding: EdgeInsets.zero,
                                title: Text(
                                  text('登录帮助', 'Sign-in help'),
                                  style: const TextStyle(fontSize: 13),
                                ),
                                children: [
                                  Text(
                                    text(
                                      '忘记密码或手机号冲突请联系平台管理员。已有展厅请使用原账号登录后绑定手机号。',
                                      'Contact the platform administrator for forgotten passwords or mobile conflicts. For an existing showroom, sign in with the original account and link a mobile number.',
                                    ),
                                  ),
                                  TextButton(
                                    onPressed: busy
                                        ? null
                                        : () => mode(setting: true),
                                    child: Text(
                                      text(
                                        '原短信账号设置密码',
                                        'Set password for original SMS account',
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                          ],
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

Future<bool> confirmMerchantUpgrade(BuildContext context) async {
  final m = ModelScope.of(context);
  if (m.account['phone_verified'] != true) {
    await Navigator.push<bool>(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen(binding: true)),
    );
    if (!context.mounted || m.account['phone_verified'] != true) return false;
  }
  final ok = await showDialog<bool>(
    context: context,
    builder: (c) => AlertDialog(
      title: Text(syncText(c, '升级为厂商账号', 'Upgrade to manufacturer')),
      content: Text(
        syncText(
          c,
          '将使用当前访客账号继续入驻，保留账号、收藏和已有资料。完善企业资料并审核通过后才会公开展厅。',
          'Continue with this visitor account and keep your favorites and data. Your showroom becomes public only after the company profile is reviewed.',
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(c, false),
          child: Text(m.t('cancel')),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(c, true),
          child: Text(syncText(c, '确认升级', 'Confirm upgrade')),
        ),
      ],
    ),
  );
  if (ok != true || !context.mounted) return false;
  final source = m.baseUrl,
      result = asMap(
        await m.api(
          '/auth/me/merchant-upgrade',
          method: 'POST',
          data: {'confirmed': true},
        ),
      );
  if (!context.mounted || source != m.baseUrl) return false;
  await m.signIn(result);
  return true;
}

Future<void> authenticateManufacturerEntry(
  BuildContext context, {
  bool register = true,
}) async {
  final m = ModelScope.of(context), source = m.baseUrl;
  if (m.token.isNotEmpty && m.account['role'] == 'visitor') {
    try {
      if ((await m.clientCapabilities())['phone_password_auth'] != true) {
        throw ApiFailure('phoneAuthUpgradeRequired');
      }
      if (!context.mounted || source != m.baseUrl) return;
      if (!await confirmMerchantUpgrade(context)) return;
      if (!context.mounted || source != m.baseUrl) return;
      m.newlyRegistered = true;
      await editMerchant(context, null, 'vendor');
    } catch (e) {
      if (context.mounted) showError(context, e);
    }
    return;
  }
  bool registered = false;
  final ok = await Navigator.push<bool>(
    context,
    MaterialPageRoute(
      builder: (_) => LoginScreen(
        register: register,
        onRegistered: () => registered = true,
      ),
    ),
  );
  if (!context.mounted || ok != true || source != m.baseUrl || !isMerchant(m)) {
    return;
  }
  if (registered) {
    m.newlyRegistered = true;
    await editMerchant(context, null, 'vendor');
  } else {
    push(context, const MerchantScreen());
  }
}
