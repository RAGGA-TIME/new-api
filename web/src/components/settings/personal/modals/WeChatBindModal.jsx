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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Input, Modal, Image } from '@douyinfe/semi-ui';
import { IconKey } from '@douyinfe/semi-icons';
import { SiWechat } from 'react-icons/si';
import { API, showError, showSuccess } from '../../../../helpers';

const WeChatBindModal = ({
  t,
  showWeChatBindModal,
  setShowWeChatBindModal,
  inputs,
  handleInputChange,
  bindWeChat,
  status,
  onBindSuccess,
}) => {
  const [qrcodeUrl, setQrcodeUrl] = useState('');
  const [sceneStr, setSceneStr] = useState('');
  const [polling, setPolling] = useState(false);
  const [loading, setLoading] = useState(false);
  const pollingRef = useRef(null);

  const isOffiAccountMode = status?.wechat_offiaccount;

  // 请求二维码
  const fetchQRCode = async () => {
    setLoading(true);
    try {
      const res = await API.post('/api/wechat/qrcode?bind=true');
      const { success, message, qrcode_url, scene_str } = res.data;
      if (success) {
        setQrcodeUrl(qrcode_url);
        setSceneStr(scene_str);
        startPolling(scene_str);
      } else {
        showError(message || '获取二维码失败');
      }
    } catch (error) {
      showError('获取微信二维码失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  // 开始轮询
  const startPolling = (scene) => {
    setPolling(true);
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
    }
    pollingRef.current = setInterval(async () => {
      try {
        const res = await API.get(`/api/wechat/scan-status?scene_str=${scene}`);
        const { success, status: scanStatus, message } = res.data;
        if (!success && scanStatus === 'expired') {
          stopPolling();
          showError('二维码已过期，请重新获取');
          return;
        }
        if (success && scanStatus === 'confirmed') {
          stopPolling();
          showSuccess(message || '绑定成功');
          setShowWeChatBindModal(false);
          if (onBindSuccess) {
            onBindSuccess();
          }
        }
      } catch (error) {
        // 继续轮询
      }
    }, 2000);
  };

  // 停止轮询
  const stopPolling = () => {
    setPolling(false);
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  };

  // 打开模态框时请求二维码
  useEffect(() => {
    if (showWeChatBindModal && isOffiAccountMode) {
      fetchQRCode();
    }
    return () => {
      stopPolling();
    };
  }, [showWeChatBindModal]);

  // 组件卸载时清除轮询
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, []);

  const handleModalClose = () => {
    stopPolling();
    setQrcodeUrl('');
    setSceneStr('');
    setShowWeChatBindModal(false);
  };

  // 新模式：动态二维码扫码绑定
  if (isOffiAccountMode) {
    return (
      <Modal
        title={
          <div className='flex items-center'>
            <SiWechat className='mr-2 text-green-500' size={20} />
            {t('绑定微信账户')}
          </div>
        }
        visible={showWeChatBindModal}
        onCancel={handleModalClose}
        footer={null}
        size={'small'}
        centered={true}
        className='modern-modal'
      >
        <div className='space-y-4 py-4 text-center'>
          {qrcodeUrl ? (
            <>
              <img
                src={qrcodeUrl}
                alt='微信二维码'
                className='mx-auto'
                style={{ width: 220, height: 220 }}
              />
              <div className='text-gray-600'>
                <p>
                  {polling
                    ? t('请使用微信扫描二维码关注公众号完成绑定')
                    : t('二维码已过期，请关闭后重新获取')}
                </p>
              </div>
            </>
          ) : (
            <div className='flex flex-col items-center py-8'>
              <p className='text-gray-400'>{t('正在获取二维码...')}</p>
            </div>
          )}
        </div>
      </Modal>
    );
  }

  // 旧模式：验证码绑定
  return (
    <Modal
      title={
        <div className='flex items-center'>
          <SiWechat className='mr-2 text-green-500' size={20} />
          {t('绑定微信账户')}
        </div>
      }
      visible={showWeChatBindModal}
      onCancel={() => setShowWeChatBindModal(false)}
      footer={null}
      size={'small'}
      centered={true}
      className='modern-modal'
    >
      <div className='space-y-4 py-4 text-center'>
        <Image src={status.wechat_qrcode} className='mx-auto' />
        <div className='text-gray-600'>
          <p>
            {t('微信扫码关注公众号，输入「验证码」获取验证码（三分钟内有效）')}
          </p>
        </div>
        <Input
          placeholder={t('验证码')}
          name='wechat_verification_code'
          value={inputs.wechat_verification_code}
          onChange={(v) => handleInputChange('wechat_verification_code', v)}
          size='large'
          className='!rounded-lg'
          prefix={<IconKey />}
        />
        <Button
          type='primary'
          theme='solid'
          size='large'
          onClick={bindWeChat}
          className='!rounded-lg w-full !bg-slate-600 hover:!bg-slate-700'
          icon={<SiWechat size={16} />}
        >
          {t('绑定')}
        </Button>
      </div>
    </Modal>
  );
};

export default WeChatBindModal;
