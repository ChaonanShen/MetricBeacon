import { Box } from '@grafana/ui';

export function WorkbenchPane(props: React.ComponentProps<typeof Box>) {
  return <Box backgroundColor="secondary" borderColor="weak" borderStyle="solid" borderRadius="md" minWidth={0} {...props} />;
}
