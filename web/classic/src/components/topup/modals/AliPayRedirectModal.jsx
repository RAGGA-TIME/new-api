/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useEffect, useState } from 'react';
import { Modal, Typography, Button, Space } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { CheckCircle, ExternalLink } from 'lucide-react';

const { Text } = Typography;

const AliPayRedirectModal = ({
  visible,
  onCancel,
  payUrl,
  tradeNo,
  amount,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('pending'); // pending | success | expired
  const [checking, setChecking] = useState(false);

  // 弹窗打开时自动跳转支付宝
  useEffect(() => {
    if (visible && payUrl) {
      window.open(payUrl, '_blank');
    }
  }, [visible, payUrl]);

  // 弹窗关闭时重置状态
  useEffect(() => {
    if (!visible) {
      setStatus('pending');
    }
  }, [visible]);

  const checkPaymentStatus = async () => {
    if (!tradeNo) return;
    setChecking(true);
    try {
      const res = await API.get(
        `/api/user/alipay/status?trade_no=${tradeNo}`,
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

  // 已完成支付
  const handlePaymentComplete = async () => {
    await checkPaymentStatus();
  };

  // 稍后再说：也校验一下支付状态，然后关闭弹窗
  const handleCheckLater = async () => {
    await checkPaymentStatus();
    if (status !== 'success') {
      onCancel?.();
    }
  };

  const handleClose = () => {
    onCancel?.();
  };

  const renderContent = () => {
    if (status === 'success') {
      return (
        <div style={{ textAlign: 'center', padding: '20px 0' }}>
          <div style={{ display: 'flex', justifyContent: 'center' }}>
            <CheckCircle size={48} color='#1677ff' style={{ marginBottom: 12 }} />
          </div>
          <Typography.Title heading={4} style={{ color: '#1677ff' }}>
            {t('支付成功')}
          </Typography.Title>
          <Text>{t('额度已到账，请查看余额')}</Text>
        </div>
      );
    }

    if (status === 'expired') {
      return (
        <div style={{ textAlign: 'center', padding: '20px 0' }}>
          <Typography.Title heading={4} style={{ color: '#f93920' }}>
            {t('订单已过期')}
          </Typography.Title>
          <Text>{t('请重新发起支付')}</Text>
        </div>
      );
    }

    return (
      <div style={{ textAlign: 'center' }}>
        <div style={{ marginTop: 8, marginBottom: 16 }}>
          <Text type='secondary'>
            {t('请在支付宝页面完成支付')}
          </Text>
        </div>
        {amount > 0 && (
          <div style={{ marginTop: 8, marginBottom: 12 }}>
            <Text>
              {t('支付金额')}：{amount} {t('元')}
            </Text>
          </div>
        )}
        {payUrl && (
          <div style={{ marginTop: 12 }}>
            <Button
              icon={<ExternalLink size={14} />}
              onClick={() => window.open(payUrl, '_blank')}
              style={{ width: '100%' }}
            >
              {t('重新打开支付宝页面')}
            </Button>
          </div>
        )}
      </div>
    );
  };

  return (
    <Modal
      title={t('支付宝支付')}
      visible={visible}
      onCancel={handleClose}
      footer={
        <Space>
          {status === 'pending' && (
            <>
              <Button
                theme='solid'
                type='primary'
                onClick={handlePaymentComplete}
                loading={checking}
              >
                {t('已完成支付')}
              </Button>
              <Button onClick={handleCheckLater} loading={checking}>
                {t('稍后再说')}
              </Button>
            </>
          )}
          <Button onClick={handleClose}>
            {status === 'success' ? t('完成') : t('关闭')}
          </Button>
        </Space>
      }
      centered
      width={400}
      maskClosable={status === 'success' || status === 'expired'}
    >
      {renderContent()}
    </Modal>
  );
};

export default AliPayRedirectModal;
