import { AppPlugin } from '@grafana/data';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { Workbench } from './workbench/Workbench';

const queryClient = new QueryClient();
export const plugin = new AppPlugin().setRootPage((props) => <QueryClientProvider client={queryClient}><Workbench {...props} /></QueryClientProvider>);
