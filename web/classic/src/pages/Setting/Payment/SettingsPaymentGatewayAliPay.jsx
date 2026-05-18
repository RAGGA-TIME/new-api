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

export default function SettingsPaymentGatewayAliPay(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('支付宝设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    AliPayEnabled: false,
    AliPayAppID: '',
    AliPayPrivateKey: '',
    AliPayPublicKey: '',
    AliPayUnitPrice: 1.0,
    AliPayMinTopUp: 1,
  });
  const [originInputs, setOriginInputs] = useState({});
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        AliPayEnabled: props.options.AliPayEnabled || false,
        AliPayAppID: props.options.AliPayAppID || '',
        AliPayPrivateKey: '',
        AliPayPublicKey: '',
        AliPayUnitPrice:
          props.options.AliPayUnitPrice !== undefined
            ? parseFloat(props.options.AliPayUnitPrice)
            : 1.0,
        AliPayMinTopUp:
          props.options.AliPayMinTopUp !== undefined
            ? parseFloat(props.options.AliPayMinTopUp)
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

  const submitAliPaySetting = async () => {
    if (props.options.ServerAddress === '') {
      showError(t('请先填写服务器地址'));
      return;
    }

    setLoading(true);
    try {
      const options = [];

      options.push({ key: 'AliPayEnabled', value: inputs.AliPayEnabled ? 'true' : 'false' });

      if (inputs.AliPayAppID !== '') {
        options.push({ key: 'AliPayAppID', value: inputs.AliPayAppID });
      }
      if (inputs.AliPayPrivateKey && inputs.AliPayPrivateKey !== '') {
        options.push({ key: 'AliPayPrivateKey', value: inputs.AliPayPrivateKey });
      }
      if (inputs.AliPayPublicKey && inputs.AliPayPublicKey !== '') {
        options.push({ key: 'AliPayPublicKey', value: inputs.AliPayPublicKey });
      }
      if (inputs.AliPayUnitPrice !== undefined && inputs.AliPayUnitPrice !== null) {
        options.push({ key: 'AliPayUnitPrice', value: inputs.AliPayUnitPrice.toString() });
      }
      if (inputs.AliPayMinTopUp !== undefined && inputs.AliPayMinTopUp !== null) {
        options.push({ key: 'AliPayMinTopUp', value: inputs.AliPayMinTopUp.toString() });
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
                /api/alipay/webhook
              </>
            }
            style={{ marginBottom: 12 }}
          />
          <Form.Switch
            field='AliPayEnabled'
            label={t('启用支付宝')}
            size='default'
            checkedText='｜'
            uncheckedText='〇'
            extraText={t('开启后用户可在充值页面使用支付宝支付，关闭后则不可使用')}
          />
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='AliPayAppID'
                label={t('AppID')}
                placeholder={t('支付宝应用 AppID')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='AliPayPublicKey'
                label={t('支付宝公钥')}
                placeholder={t('留空表示保持当前不变')}
                extraText={t('支付宝公钥，用于验签，保存后不会回显')}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.TextArea
                field='AliPayPrivateKey'
                label={t('应用私钥')}
                placeholder={t('留空表示保持当前不变')}
                extraText={t('应用 RSA2 私钥，保存后不会回显')}
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
                field='AliPayUnitPrice'
                precision={2}
                label={t('充值汇率（x元/美金）')}
                placeholder={t('例如：7，就是7元/美金')}
                extraText={t('1 美元对应的人民币金额，设为 7 表示 7:1')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='AliPayMinTopUp'
                label={t('最低充值金额（元）')}
                placeholder={t('例如：1，就是最低充值1元')}
                extraText={t('用户单次最少可充值的人民币金额')}
              />
            </Col>
          </Row>
          <Button onClick={submitAliPaySetting} style={{ marginTop: 16 }}>
            {t('更新支付宝设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
