import React, { useEffect, useState, useRef } from 'react';
import { Modal, Typography, Button, Space } from '@douyinfe/semi-ui';
import { QRCodeSVG } from 'qrcode.react';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { CheckCircle, Clock, XCircle } from 'lucide-react';

const { Text } = Typography;

const WeChatPayQRCodeModal = ({
  visible,
  onCancel,
  codeUrl,
  tradeNo,
  amount,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('pending'); // pending | success | expired
  const [countdown, setCountdown] = useState(300); // 5 minutes
  const [checking, setChecking] = useState(false);
  const timerRef = useRef(null);

  useEffect(() => {
    if (!visible || !tradeNo) return;

    setStatus('pending');
    setCountdown(300);

    timerRef.current = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          setStatus('expired');
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [visible, tradeNo]);

  useEffect(() => {
    if ((status === 'success' || status === 'expired') && timerRef.current) {
      clearInterval(timerRef.current);
    }
  }, [status]);

  const formatCountdown = (seconds) => {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  };

  const handleClose = () => {
    if (timerRef.current) clearInterval(timerRef.current);
    onCancel?.();
  };

  const handlePaymentComplete = async () => {
    if (!tradeNo) return;
    setChecking(true);
    try {
      const res = await API.get(
        `/api/user/wechat-pay/status?trade_no=${tradeNo}`,
      );
      const { message, data } = res.data;
      if (message === 'success') {
        if (data.status === 'success') {
          setStatus('success');
          showSuccess(t('充值成功'));
          onSuccess?.();
        } else if (data.status === 'expired') {
          setStatus('expired');
        } else {
          showError(t('支付尚未完成，请稍后再试'));
        }
      } else {
        showError(t('查询支付状态失败'));
      }
    } catch (e) {
      showError(t('查询支付状态失败'));
    } finally {
      setChecking(false);
    }
  };

  const renderContent = () => {
    if (status === 'success') {
      return (
        <div style={{ textAlign: 'center', padding: '20px 0' }}>
          <CheckCircle size={48} color='#07C160' style={{ marginBottom: 12 }} />
          <Typography.Title heading={4} style={{ color: '#07C160' }}>
            {t('支付成功')}
          </Typography.Title>
          <Text>{t('额度已到账，请查看余额')}</Text>
        </div>
      );
    }

    if (status === 'expired') {
      return (
        <div style={{ textAlign: 'center', padding: '20px 0' }}>
          <XCircle size={48} color='#f93920' style={{ marginBottom: 12 }} />
          <Typography.Title heading={4} style={{ color: '#f93920' }}>
            {t('二维码已过期')}
          </Typography.Title>
          <Text>{t('请重新发起支付')}</Text>
        </div>
      );
    }

    return (
      <div style={{ textAlign: 'center' }}>
        {codeUrl && (
          <div
            style={{
              display: 'inline-block',
              padding: 16,
              background: 'white',
              borderRadius: 8,
              border: '1px solid #e0e0e0',
            }}
          >
            <QRCodeSVG value={codeUrl} size={200} />
          </div>
        )}
        <div style={{ marginTop: 16 }}>
          <Text type='secondary'>
            {t('请使用微信扫描二维码完成支付')}
          </Text>
        </div>
        {amount > 0 && (
          <div style={{ marginTop: 8 }}>
            <Text>
              {t('支付金额')}：{amount} {t('元')}
            </Text>
          </div>
        )}
        <div
          style={{
            marginTop: 12,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 6,
          }}
        >
          <Clock size={14} />
          <Text type='secondary'>
            {t('剩余时间')}：{formatCountdown(countdown)}
          </Text>
        </div>
      </div>
    );
  };

  return (
    <Modal
      title={t('微信支付')}
      visible={visible}
      onCancel={handleClose}
      footer={
        <Space>
          {status === 'pending' && (
            <Button
              theme='solid'
              type='primary'
              onClick={handlePaymentComplete}
              loading={checking}
            >
              {t('已完成支付')}
            </Button>
          )}
          <Button onClick={handleClose}>
            {status === 'success' ? t('完成') : t('关闭')}
          </Button>
        </Space>
      }
      centered
      width={360}
      maskClosable={status === 'success' || status === 'expired'}
    >
      {renderContent()}
    </Modal>
  );
};

export default WeChatPayQRCodeModal;
