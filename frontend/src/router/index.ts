import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import HomePage from '../views/HomePage.vue';
import IdentityPage from '../views/IdentityPage.vue';
import ProposalListPage from '../views/ProposalListPage.vue';
import ProposalDetailPage from '../views/ProposalDetailPage.vue';
import AdminPage from '../views/AdminPage.vue';

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: HomePage,
  },
  {
    path: '/identity',
    name: 'identity',
    component: IdentityPage,
  },
  {
    path: '/proposals',
    name: 'proposals',
    component: ProposalListPage,
  },
  {
    path: '/proposals/:id',
    name: 'proposal-detail',
    component: ProposalDetailPage,
    props: true,
  },
  {
    path: '/admin',
    name: 'admin',
    component: AdminPage,
  },
  {
  path: '/user-center',
  name: 'user-center',
  component: () => import('../views/UserCenterPage.vue')
}
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;

