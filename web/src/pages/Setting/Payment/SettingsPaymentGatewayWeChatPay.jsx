import React, { useEffect, useState, useRef } from 'react';
import { Banner, Button, Form, Row, Col, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { BookOpen } from 'lucide-react';

export default function SettingsPaymentGatewayWeChatPay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('微信支付设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    WeChatPayAppID: '',
    WeChatPayMchID: '',
    WeChatPayAPIv3Key: '',
    WeChatPayPrivateKey: '',
    WeChatPaySerialNo: '',
    WeChatPayPublicKeyID: '',
    WeChatPayPublicKey: '',
    WeChatPayMinTopUp: 1,
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        WeChatPayAppID: props.options.WeChatPayAppID || '',
        WeChatPayMchID: props.options.WeChatPayMchID || '',
        WeChatPayAPIv3Key: '',
        WeChatPayPrivateKey: '',
        WeChatPaySerialNo: props.options.WeChatPaySerialNo || '',
        WeChatPayPublicKeyID: props.options.WeChatPayPublicKeyID || '',
        WeChatPayPublicKey: '',
        WeChatPayMinTopUp:
          props.options.WeChatPayMinTopUp !== undefined
            ? parseFloat(props.options.WeChatPayMinTopUp)
            : 1,
      };
      setInputs(currentInputs);
      setOriginInputs({ ...currentInputs });
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitWeChatPaySetting = async () => {
    if (props.options.ServerAddress === '') {
      showError(t('请先填写服务器地址'));
      return;
    }

    setLoading(true);
    try {
      const options = [];

      if (inputs.WeChatPayAppID !== '') {
        options.push({ key: 'WeChatPayAppID', value: inputs.WeChatPayAppID });
      }
      if (inputs.WeChatPayMchID !== '') {
        options.push({ key: 'WeChatPayMchID', value: inputs.WeChatPayMchID });
      }
      if (inputs.WeChatPayAPIv3Key && inputs.WeChatPayAPIv3Key !== '') {
        options.push({ key: 'WeChatPayAPIv3Key', value: inputs.WeChatPayAPIv3Key });
      }
      if (inputs.WeChatPayPrivateKey && inputs.WeChatPayPrivateKey !== '') {
        options.push({ key: 'WeChatPayPrivateKey', value: inputs.WeChatPayPrivateKey });
      }
      if (inputs.WeChatPaySerialNo !== '') {
        options.push({ key: 'WeChatPaySerialNo', value: inputs.WeChatPaySerialNo });
      }
      if (inputs.WeChatPayPublicKeyID !== '') {
        options.push({ key: 'WeChatPayPublicKeyID', value: inputs.WeChatPayPublicKeyID });
      }
      if (inputs.WeChatPayPublicKey && inputs.WeChatPayPublicKey !== '') {
        options.push({ key: 'WeChatPayPublicKey', value: inputs.WeChatPayPublicKey });
      }
      if (inputs.WeChatPayMinTopUp !== undefined && inputs.WeChatPayMinTopUp !== null) {
        options.push({ key: 'WeChatPayMinTopUp', value: inputs.WeChatPayMinTopUp.toString() });
      }

      const requestQueue = options.map((opt) =>
        API.put('/api/option/', {
          key: opt.key,
          value: opt.value,
        }),
      );

      const results = await Promise.all(requestQueue);

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => {
          showError(res.data.message);
        });
      } else {
        showSuccess(t('更新成功'));
        setOriginInputs({ ...inputs });
        props.refresh?.();
      }
    } catch (error) {
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<BookOpen size={16} />}
            description={
              <>
                {t('回调地址')}：
                {props.options.ServerAddress
                  ? removeTrailingSlash(props.options.ServerAddress)
                  : t('网站地址')}
                /api/wechat-pay/webhook
              </>
            }
            style={{ marginBottom: 12 }}
          />
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayAppID'
                label={t('AppID')}
                placeholder={t('微信公众号/小程序的 AppID')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayMchID'
                label={t('商户号')}
                placeholder={t('微信支付商户号')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPaySerialNo'
                label={t('商户证书序列号')}
                placeholder={t('商户API证书序列号')}
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayPublicKeyID'
                label={t('微信支付公钥ID')}
                placeholder={t('微信支付公钥ID，以 pub_key_id_ 开头')}
                extraText={t('在商户平台 API安全 页获取，配置后使用公钥模式验签')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='WeChatPayPublicKey'
                label={t('微信支付公钥')}
                placeholder={t('PEM 格式，留空表示保持当前不变')}
                extraText={t('微信支付公钥，保存后不会回显。未配置则使用平台证书模式')}
                type='password'
                autosize
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='WeChatPayAPIv3Key'
                label={t('APIv3 密钥')}
                placeholder={t('留空表示保持当前不变')}
                extraText={t('用于回调通知解密，保存后不会回显')}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='WeChatPayPrivateKey'
                label={t('商户私钥')}
                placeholder={t('PEM 格式，留空表示保持当前不变')}
                extraText={t('商户API证书私钥，保存后不会回显')}
                type='password'
                autosize
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='WeChatPayMinTopUp'
                label={t('最低充值金额（元）')}
                placeholder={t('例如：1，就是最低充值1元')}
                extraText={t('用户单次最少可充值的人民币金额')}
              />
            </Col>
          </Row>
          <Button onClick={submitWeChatPaySetting} style={{ marginTop: 16 }}>
            {t('更新微信支付设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
