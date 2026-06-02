package sdk

import (
	"context"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
)

// TestNewChainMakerClient_MissingSignKey 测试缺少签名私钥时应返回错误
func TestNewChainMakerClient_MissingSignKey(t *testing.T) {
	conf := ChainMakerConf{
		ChainId:  "chain1",
		AuthType: "public",
		HashType: "SHA256",
		SignKey:  "", // 缺少签名私钥
		Nodes: []ChainMakerNodeConf{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	_, err := NewChainMakerClient(context.Background(), "test-chain", conf, nil, defaultLogConf(), nil)
	if err == nil {
		t.Fatal("expected error for missing sign_key")
	}
	if !containsStr(err.Error(), "sign_key is required") {
		t.Fatalf("expected error about sign_key, got: %v", err)
	}
}

// TestNewChainMakerClient_CertModeMissingSignCert 测试证书模式下缺少签名证书时应返回错误
func TestNewChainMakerClient_CertModeMissingSignCert(t *testing.T) {
	conf := ChainMakerConf{
		ChainId:  "chain1",
		AuthType: "permissionedwithcert",
		OrgId:    "org1",
		HashType: "SHA256",
		SignKey:  "c29tZS1mYWtlLWtleQ==", // base64("some-fake-key")
		SignCert: "",                     // 缺少签名证书
		Nodes: []ChainMakerNodeConf{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	_, err := NewChainMakerClient(context.Background(), "test-chain", conf, nil, defaultLogConf(), nil)
	if err == nil {
		t.Fatal("expected error for missing sign_cert in cert mode")
	}
	if !containsStr(err.Error(), "sign_cert is required") {
		t.Fatalf("expected error about sign_cert, got: %v", err)
	}
}

// TestNewChainMakerClient_EmptyNodes 测试空节点列表时 CreateSDKClient 应返回错误
func TestNewChainMakerClient_EmptyNodes(t *testing.T) {
	conf := ChainMakerConf{
		ChainId:  "chain1",
		AuthType: "public",
		HashType: "SHA256",
		SignKey:  "c29tZS1mYWtlLWtleQ==", // base64("some-fake-key")
		Nodes:    []ChainMakerNodeConf{}, // 空节点列表
	}

	_, err := NewChainMakerClient(context.Background(), "test-chain", conf, nil, defaultLogConf(), nil)
	// 空节点列表会导致 SDK 创建失败
	if err == nil {
		t.Fatal("expected error for empty nodes")
	}
}

// 辅助函数
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func defaultLogConf() logx.LogConf {
	return logx.LogConf{
		Path:     "/tmp/test-logs",
		KeepDays: 7,
	}
}
