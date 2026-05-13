import { useEffect, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { useUpdateOption } from '../hooks/use-update-option'

export interface AliPaySettingsValues {
  AliPayEnabled: boolean
  AliPayAppID: string
  AliPayPrivateKey: string
  AliPayPublicKey: string
  AliPayUnitPrice: number
  AliPayMinTopUp: number
}

interface Props {
  defaultValues: AliPaySettingsValues
}

export function AliPaySettingsSection({ defaultValues }: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const initialRef = useRef(defaultValues)
  const signature = JSON.stringify(defaultValues)

  const form = useForm<AliPaySettingsValues>({
    defaultValues,
  })

  useEffect(() => {
    const parsed = JSON.parse(signature) as AliPaySettingsValues
    initialRef.current = parsed
    form.reset(parsed)
  }, [signature, form])

  const onSave = async () => {
    const values = form.getValues()
    const updates: Array<{ key: string; value: string | number | boolean }> = []

    if (values.AliPayEnabled !== initialRef.current.AliPayEnabled) {
      updates.push({ key: 'AliPayEnabled', value: values.AliPayEnabled })
    }

    if (values.AliPayAppID !== initialRef.current.AliPayAppID) {
      updates.push({ key: 'AliPayAppID', value: values.AliPayAppID })
    }

    if (values.AliPayPrivateKey && values.AliPayPrivateKey !== initialRef.current.AliPayPrivateKey) {
      updates.push({ key: 'AliPayPrivateKey', value: values.AliPayPrivateKey })
    }

    if (values.AliPayPublicKey && values.AliPayPublicKey !== initialRef.current.AliPayPublicKey) {
      updates.push({ key: 'AliPayPublicKey', value: values.AliPayPublicKey })
    }

    if (values.AliPayUnitPrice !== initialRef.current.AliPayUnitPrice) {
      updates.push({ key: 'AliPayUnitPrice', value: values.AliPayUnitPrice })
    }

    if (values.AliPayMinTopUp !== initialRef.current.AliPayMinTopUp) {
      updates.push({ key: 'AliPayMinTopUp', value: values.AliPayMinTopUp })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    initialRef.current = { ...values }
    toast.success(t('Alipay settings saved'))
  }

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('Alipay Gateway')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t('Configuration for Alipay Page Pay integration')}
        </p>
      </div>

      <div className='rounded-md bg-blue-50 p-4 text-sm text-blue-900 dark:bg-blue-950 dark:text-blue-100'>
        <p className='mb-2 font-medium'>{t('Webhook Configuration:')}</p>
        <ul className='list-inside list-disc space-y-1'>
          <li>
            {t('Webhook URL:')}{' '}
            <code className='rounded bg-blue-100 px-1 py-0.5 text-xs dark:bg-blue-900'>
              {'<ServerAddress>/api/alipay/webhook'}
            </code>
          </li>
          <li>
            {t('Sign type:')}{' '}
            <code className='rounded bg-blue-100 px-1 py-0.5 text-xs dark:bg-blue-900'>
              RSA2
            </code>
          </li>
          <li>
            {t('Configure at:')}{' '}
            <a
              href='https://open.alipay.com'
              target='_blank'
              rel='noreferrer'
              className='underline hover:no-underline'
            >
              {t('Alipay Open Platform')}
            </a>
          </li>
        </ul>
      </div>

      <Form {...form}>
        <div className='space-y-6'>
          <FormField
            control={form.control}
            name='AliPayEnabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Enable Alipay')}
                  </FormLabel>
                  <FormDescription>
                    {t('Enable Alipay as a payment method for top-up')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AliPayAppID'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('App ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='2021...'
                      autoComplete='off'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Alipay application ID from Open Platform')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AliPayPublicKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Alipay Public Key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('Enter Alipay public key')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Alipay public key for signature verification (leave blank unless updating)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='AliPayPrivateKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Application Private Key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Enter application private key')}
                    autoComplete='new-password'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Your application RSA2 private key (leave blank unless updating)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AliPayUnitPrice'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Unit price (CNY / USD)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.01'
                      min={0}
                      value={(field.value ?? 1) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('e.g., 7 means 7 CNY per USD')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AliPayMinTopUp'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Minimum top-up (CNY)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='1'
                      min={1}
                      value={(field.value ?? 1) as number}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Minimum recharge amount in CNY')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button
            type='button'
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              onSave()
            }}
            disabled={updateOption.isPending}
          >
            {updateOption.isPending
              ? t('Saving...')
              : t('Save Alipay settings')}
          </Button>
        </div>
      </Form>
    </div>
  )
}
