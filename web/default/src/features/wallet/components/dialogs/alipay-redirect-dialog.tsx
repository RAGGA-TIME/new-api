import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { ExternalLink, CheckCircle2, Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { getAliPayOrderStatus } from '../../api'

interface AliPayRedirectDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  payUrl: string
  tradeNo: string
  paymentAmount: number
  onSuccess: () => void
}

export function AliPayRedirectDialog({
  open,
  onOpenChange,
  payUrl,
  tradeNo,
  paymentAmount,
  onSuccess,
}: AliPayRedirectDialogProps) {
  const { t } = useTranslation()
  const [checking, setChecking] = useState(false)
  const [paymentSuccess, setPaymentSuccess] = useState(false)

  // Auto-redirect to Alipay when dialog opens
  useEffect(() => {
    if (open && payUrl) {
      window.open(payUrl, '_blank')
    }
  }, [open, payUrl])

  // Reset state when dialog closes
  useEffect(() => {
    if (!open) {
      setPaymentSuccess(false)
    }
  }, [open])

  const checkPaymentStatus = useCallback(async () => {
    if (!tradeNo) return

    try {
      setChecking(true)
      const response = await getAliPayOrderStatus(tradeNo)
      if (response.data?.status === 'success') {
        setPaymentSuccess(true)
        onSuccess()
      }
    } catch {
      // Silently ignore errors during polling
    } finally {
      setChecking(false)
    }
  }, [tradeNo, onSuccess])

  // Auto-poll every 3 seconds
  useEffect(() => {
    if (!open || paymentSuccess || !tradeNo) return

    const interval = setInterval(checkPaymentStatus, 3000)
    return () => clearInterval(interval)
  }, [open, paymentSuccess, tradeNo, checkPaymentStatus])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-md'>
        <DialogHeader>
          <DialogTitle className='text-xl font-semibold'>
            {paymentSuccess
              ? t('Payment Successful')
              : t('Alipay Payment')}
          </DialogTitle>
          <DialogDescription>
            {paymentSuccess
              ? t('Your payment has been confirmed and balance has been updated.')
              : t('Complete your payment on the Alipay page')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          {paymentSuccess ? (
            <div className='flex flex-col items-center gap-3 py-6'>
              <CheckCircle2 className='h-16 w-16 text-green-500' />
              <p className='text-lg font-medium text-green-600'>
                {t('Payment Successful')}
              </p>
            </div>
          ) : (
            <>
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Payment Amount')}
                </span>
                <span className='text-lg font-semibold'>
                  ¥{paymentAmount.toFixed(2)}
                </span>
              </div>

              <div className='bg-muted/50 rounded-lg p-4'>
                <div className='flex items-center gap-2'>
                  <Loader2 className='h-4 w-4 animate-spin text-blue-500' />
                  <span className='text-sm'>
                    {t('Waiting for payment...')}
                  </span>
                </div>
                <p className='text-muted-foreground mt-2 text-xs'>
                  {t('After completing payment on Alipay, this page will automatically update.')}
                </p>
              </div>

              <div className='flex flex-col gap-2'>
                <Button
                  variant='outline'
                  onClick={() => window.open(payUrl, '_blank')}
                  className='w-full'
                >
                  <ExternalLink className='mr-2 h-4 w-4' />
                  {t('Open Alipay Page')}
                </Button>

                <Button
                  variant='ghost'
                  onClick={checkPaymentStatus}
                  disabled={checking}
                  className='w-full'
                >
                  {checking && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                  {t('Check Payment Status')}
                </Button>
              </div>
            </>
          )}
        </div>

        <DialogFooter>
          <Button
            onClick={() => onOpenChange(false)}
            variant={paymentSuccess ? 'default' : 'outline'}
          >
            {paymentSuccess ? t('Done') : t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
